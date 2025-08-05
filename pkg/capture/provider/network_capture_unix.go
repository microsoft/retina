//go:build unix

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package provider

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"go.uber.org/zap"

	captureConstants "github.com/microsoft/retina/pkg/capture/constants"
	"github.com/microsoft/retina/pkg/capture/file"
	"github.com/microsoft/retina/pkg/log"
)

type iptablesMode string

const (
	legacyIptablesMode iptablesMode = "legacy"
	nftIptablesMode    iptablesMode = "nft"
)

var (
	errTcpdumpCommandNotConstructed = errors.New("tcpdump command is not constructed with expected arguments")
	errTcpdumpStopFailed            = errors.New("tcpdump stop failed")
	errIptablesUnavilable           = errors.New("no iptables command is available")
)

// constructTcpdumpCommand creates a tcpdump command with the appropriate arguments
// based on environment variables for raw filter, specific interfaces, or default behavior
func constructTcpdumpCommand(captureFilePath string) *exec.Cmd {
	// NOTE(mainred): The tcpdump release of debian:bullseye image, which is for preparing clang and tools, runs as
	// tcpdump user by default for savefiles for output, but when the binary and library are copied to the distroless
	// base image, we lost tcpdump user, and the following error will be raised when running tcpdump in our capture pod.
	// tcpdump: Couldn't find user 'tcpdump'
	// To disable this behavior, we use `--relinquish-privileges=root` same as `-Z root`.
	// ref: https://manpages.debian.org/bullseye/tcpdump/tcpdump.8.en.html#Z
	captureStartCmd := exec.Command(
		"tcpdump",
		"-w", captureFilePath,
		"--relinquish-privileges=root",
	)

	if packetSize := os.Getenv(captureConstants.PacketSizeEnvKey); packetSize != "" {
		captureStartCmd.Args = append(
			captureStartCmd.Args,
			"-s", packetSize,
		)
	}

	// If we set flag and value into the arg item of args, the space between flag and value will not treated as part of
	// value, for example, "-i eth0" will be treated as "-i" and " eth0", thus brings a tcpdump unknown interface error.
	if tcpdumpRawFilter := os.Getenv(captureConstants.TcpdumpRawFilterEnvKey); tcpdumpRawFilter != "" {
		tcpdumpRawFilterSlice := strings.Split(tcpdumpRawFilter, " ")
		captureStartCmd.Args = append(captureStartCmd.Args, tcpdumpRawFilterSlice...)
	} else if specificInterfaces := os.Getenv(captureConstants.CaptureInterfacesEnvKey); specificInterfaces != "" {
		// Use specific interfaces if provided
		interfaceList := strings.Split(specificInterfaces, ",")
		for _, iface := range interfaceList {
			iface = strings.TrimSpace(iface)
			if iface != "" {
				captureStartCmd.Args = append(captureStartCmd.Args, "-i", iface)
			}
		}
	} else {
		// Default to capturing on all interfaces if no raw tcpdump filter or specific interfaces are specified
		captureStartCmd.Args = append(captureStartCmd.Args, "-i", "any")
	}

	return captureStartCmd
}

type NetworkCaptureProvider struct {
	NetworkCaptureProviderCommon
	TmpCaptureDir string
	Filename      file.CaptureFilename

	l *log.ZapLogger
}

var _ NetworkCaptureProviderInterface = &NetworkCaptureProvider{}

func NewNetworkCaptureProvider(logger *log.ZapLogger) NetworkCaptureProviderInterface {
	return &NetworkCaptureProvider{
		NetworkCaptureProviderCommon: NetworkCaptureProviderCommon{l: logger},
		l:                            logger,
	}
}

func (ncp *NetworkCaptureProvider) Setup(filename file.CaptureFilename) (string, error) {
	captureFolderDir, err := ncp.NetworkCaptureProviderCommon.Setup(filename)
	if err != nil {
		return "", err
	}
	ncp.l.Info("Created temporary folder for network capture", zap.String("capture temporary folder", captureFolderDir))

	ncp.TmpCaptureDir = captureFolderDir
	ncp.Filename = filename

	return ncp.TmpCaptureDir, nil
}

func (ncp *NetworkCaptureProvider) CaptureNetworkPacket(ctx context.Context, filter string, duration, maxSizeMB int) error {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(duration)*time.Second)
	defer cancel()

	captureFileName := ncp.Filename.String() + ".pcap"
	captureFilePath := filepath.Join(ncp.TmpCaptureDir, captureFileName)

	// Remove the folder in case it already exists to mislead the file size check.
	os.Remove(captureFilePath) //nolint:errcheck // File may not exist, ok to ignore error

	// NOTE(mainred): The tcpdump release of debian:bullseye image, which is for preparing clang and tools, runs as
	// tcpdump user by default for savefiles for output, but when the binary and library are copied to the distroless
	// base image, we lost tcpdump user, and the following error will be raised when running tcpdump in our capture pod.
	// tcpdump: Couldn't find user 'tcpdump'
	// To disable this behavior, we use `--relinquish-privileges=root` same as `-Z root`.
	// ref: https://manpages.debian.org/bullseye/tcpdump/tcpdump.8.en.html#Z
	captureStartCmd := constructTcpdumpCommand(captureFilePath)

	if filter != "" {
		captureStartCmd.Args = append(
			captureStartCmd.Args,
			filter,
		)
	}

	ncp.l.Info("Running tcpdump with args", zap.String("tcpdump command", captureStartCmd.String()), zap.Any("tcpdump args", captureStartCmd.Args))

	tcpdumpLogFile, err := ncp.NetworkCaptureProviderCommon.networkCaptureCommandLog("tcpdump.log", captureStartCmd)
	if err != nil {
		return err
	}

	// Store tcpdpump log as part of capture artifacts.
	defer func() {
		if tcpdumpLog, err := os.ReadFile(tcpdumpLogFile.Name()); err != nil {
			ncp.l.Warn("Failed to read tcpdump log", zap.Error(err))
		} else {
			ncp.l.Info("Tcpdump command output: " + string(tcpdumpLog))
		}
		tcpdumpLogFile.Close()
	}()

	err = captureStartCmd.Start()
	if err != nil {
		ncp.l.Error("Failed to start tcpdump", zap.Error(err))
		return err
	}

	doneChan := make(chan bool)
	errChan := make(chan error)

	// NOTE(mainred): We tried to use `-W=1` plus `-G=$Duration` to exit tcpdump, but when the rotate duration,
	// specified by `-G`, reaches, tcpdump does not stop and capture file is rotated somehow.
	if duration != 0 {
		ncp.l.Info(fmt.Sprintf("Tcpdump will stop after %v seconds", duration))
		go func() {
			time.Sleep(time.Second * time.Duration(duration))
			doneChan <- true
		}()
	}

	// TODO(mainred): make check interval configurable.
	fileSizeCheckIntervalInSecond := 5
	// Tcpdump cannot stop when a specified size reaches, so we check the capture file size with a const time interval,
	// and stop tcpdump process when the file size meets the requirement.
	if maxSizeMB != 0 {
		ncp.l.Info(fmt.Sprintf("Tcpdump will stop when the capture file size reaches %dMB.", maxSizeMB))
		go func() {
			// Chances are that the capture file is not created when we check the file size.
			time.Sleep(time.Second * time.Duration(fileSizeCheckIntervalInSecond))
			captureFile, err := os.Open(captureFilePath)
			if err != nil {
				ncp.l.Error("Failed to open capture file", zap.String("capture file path", captureFilePath), zap.Error(err))
				ncp.l.Error("Please make sure tcpdump command is constructed with expected arguments", zap.String("tcpdump args", fmt.Sprintf("%+q", captureStartCmd.Args)))
				errChan <- errTcpdumpCommandNotConstructed
				return
			}

			for {
				fileStat, err := captureFile.Stat()
				if err != nil {
					ncp.l.Error("Failed to get capture file info", zap.String("capture file path", captureFilePath), zap.Error(err))
					continue
				}
				fileSizeBytes := fileStat.Size()
				if int(fileSizeBytes) > maxSizeMB*1024*1224 {
					break
				}

				time.Sleep(time.Second * time.Duration(fileSizeCheckIntervalInSecond))
			}
			doneChan <- true
		}()
	}

	select {
	case <-doneChan:
	case <-ctx.Done():
		ncp.l.Info("Tcpdump will be stopped - got OS signal, or timeout reached", zap.Error(ctx.Err()))
	case err := <-errChan:
		return err
	}
	ncp.l.Info("Stop tcpdump")
	// Kill signal will not wait until the process has actually existed, thus the captured network packets may not be
	// flushed to the capture file. Instead, we signal terminate and wait until the process to exit.
	if err := captureStartCmd.Process.Signal(syscall.SIGTERM); err != nil {
		ncp.l.Error("Failed to signal terminate to process, will kill the process", zap.Error(err))
		if killErr := captureStartCmd.Process.Kill(); killErr != nil {
			return fmt.Errorf("%w: %w", errTcpdumpStopFailed, killErr)
		}
		return err
	}
	if _, err = captureStartCmd.Process.Wait(); err != nil {
		ncp.l.Error("Failed to wait for the process to exit", zap.Error(err))
		return err
	}

	return nil
}

type command struct {
	name          string
	args          []string
	description   string
	ignoreFailure bool
}

func (ncp *NetworkCaptureProvider) CollectMetadata() error {
	ncp.l.Info("Start to collect network metadata")

	iptablesMode, err := obtainIptablesMode(ncp.l)
	if err != nil {
		return fmt.Errorf("failed to determine iptables modes. %w", err)
	}

	ncp.l.Info(fmt.Sprintf("Iptables mode %s is used", iptablesMode))
	iptablesSaveCmdName := fmt.Sprintf("iptables-%s-save", iptablesMode)
	iptablesCmdName := fmt.Sprintf("iptables-%s", iptablesMode)

	metadataList := []struct {
		commands []command
		fileName string
	}{
		{
			commands: []command{
				{
					name:        "ip",
					args:        []string{"-d", "-j", "addr", "show"},
					description: "IP address configuration",
				},
				{
					name:        "ip",
					args:        []string{"-d", "-j", "neighbor", "show"},
					description: "IP neighbor status",
				},
				{
					name:        "ip",
					args:        []string{"rule", "list"},
					description: "Policy routing list",
				},
				{
					name:        "ip",
					args:        []string{"route", "show", "table", "all"},
					description: "Routes of all route tables",
				},
			},
			fileName: "ip-resources.txt",
		},
		{
			commands: []command{
				{
					name:        iptablesSaveCmdName,
					description: "IPtables rules",
				},
				{
					name:        iptablesCmdName,
					args:        []string{"-vnx", "-L"},
					description: "IPtables rules and stats in filter table",
				},
				{
					name:        iptablesCmdName,
					args:        []string{"-vnx", "-L", "-t", "nat"},
					description: "IPtables rules and stats in nat table",
				},
				{
					name:        iptablesCmdName,
					args:        []string{"-vnx", "-L", "-t", "mangle"},
					description: "IPtables rules and stats in mangle table",
				},
			},
			fileName: "iptables-rules.txt",
		},
		{
			commands: []command{
				{
					name:        "ss",
					args:        []string{"-s"},
					description: "Socket statistics summary",
				},
				{
					name:        "ss",
					args:        []string{"-tapionume"},
					description: "Socket statistics details",
				},
			},
			fileName: "socket-stats.txt",
		},
		{
			commands: []command{
				{
					name: "cp",
					// '/proc/net' is a symbolic link to /proc/self/net since Linux 2.6.25 to honor network stack
					// virtualization as the advent of network namespaces.
					// https://man7.org/linux/man-pages/man5/proc.5.html
					// NOTE(qinhao): We now clone only node host network net(self) stats here even when the capture
					// target is container(s), for simplicity.
					args:        []string{"-r", "/proc/self/net", filepath.Join(ncp.TmpCaptureDir, "proc-net")},
					description: "networking stats",
				},
				{
					name:        "cp",
					args:        []string{"-r", "/proc/sys/net", filepath.Join(ncp.TmpCaptureDir, "proc-sys-net")},
					description: "kernel networking configuration",
					// Errors will occur when copying kernel networking configuration for not all files under /proc/sys/net are
					// readable, like '/proc/sys/net/ipv4/route/flush', which doesn't implement the read function.
					ignoreFailure: true,
				},
			},
		},
	}

	for _, metadata := range metadataList {
		if metadata.fileName != "" {
			ncp.processMetadataFile(metadata)
		} else {
			for _, command := range metadata.commands {
				cmd := exec.Command(command.name, command.args...) // nolint:gosec // no sensitive data
				// Errors will when copying kernel networking configuration for not all files under /proc/sys/net are
				// readable, like '/proc/sys/net/ipv4/route/flush', which doesn't implement the read function.
				if output, err := cmd.CombinedOutput(); err != nil && !command.ignoreFailure {
					// Don't return for error to continue capturing following network metadata.
					ncp.l.Error("Failed to execute command", zap.String("command", cmd.String()), zap.String("output", string(output)), zap.Error(err))
				}
			}
		}
	}

	ncp.l.Info("Done for collecting network metadata")

	return nil
}

func (ncp *NetworkCaptureProvider) processMetadataFile(metadata struct {
	commands []command
	fileName string
},
) {
	captureMetadataFilePath := filepath.Join(ncp.TmpCaptureDir, metadata.fileName)
	outfile, err := os.OpenFile(captureMetadataFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		ncp.l.Error("Failed to create metadata file", zap.String("metadata file path", captureMetadataFilePath), zap.Error(err))
		return
	}
	defer outfile.Close()

	if _, err := outfile.WriteString("Summary:\n\n"); err != nil {
		ncp.l.Error("Failed to write summary to file", zap.String("file", outfile.Name()), zap.Error(err))
	}

	// Print headlines for all commands in output file.
	cmds := []*exec.Cmd{}
	for _, command := range metadata.commands {
		cmd := exec.Command(command.name, command.args...) // nolint:gosec // no sensitive data
		cmds = append(cmds, cmd)
		commandSummary := fmt.Sprintf("%s(%s)\n", cmd.String(), command.description)
		if _, err := outfile.WriteString(commandSummary); err != nil {
			ncp.l.Error("Failed to write command description to file", zap.String("file", outfile.Name()), zap.Error(err))
		}
	}

	if _, err := outfile.WriteString("\nExecute:\n\n"); err != nil {
		ncp.l.Error("Failed to write command output to file", zap.String("file", outfile.Name()), zap.Error(err))
	}

	// Write command stdout and stderr to output file
	for _, cmd := range cmds {
		if _, err := fmt.Fprintf(outfile, "%s\n\n", cmd.String()); err != nil {
			ncp.l.Error("Failed to write string to file", zap.String("file", outfile.Name()), zap.Error(err))
		}

		cmd.Stdout = outfile
		cmd.Stderr = outfile
		if err := cmd.Run(); err != nil {
			// Don't return for error to continue capturing following network metadata.
			ncp.l.Error("Failed to execute command", zap.String("command", cmd.String()), zap.Error(err))
			// Log the error in output file because this error does not stop capture job pod from finishing,
			// and the job can be recycled automatically leaving no info to debug.
			if _, err = fmt.Fprintf(outfile, "Failed to run %q, error: %s)", cmd.String(), err.Error()); err != nil {
				ncp.l.Error("Failed to write command run failure", zap.String("command", cmd.String()), zap.Error(err))
			}
		}
	}
}

func (ncp *NetworkCaptureProvider) Cleanup() error {
	ncp.l.Info("Cleanup network capture", zap.String("capture name", ncp.Filename.CaptureName), zap.String("temporary dir", ncp.TmpCaptureDir))
	ncp.NetworkCaptureProviderCommon.Cleanup()
	return nil
}

// obtainIptablesMode return the available iptables mode, and returns empty when no iptables is available.
func obtainIptablesMode(logger *log.ZapLogger) (iptablesMode, error) {
	// Since iptables v1.8, nf_tables are introduced as an improvement of legacy iptables, but provides the same user
	// interface as legacy iptables through iptables-nft command.
	// based on: https://github.com/kubernetes-sigs/iptables-wrappers/blob/97b01f43a8e8db07840fc4b95e833a37c0d36b12/iptables-wrapper-installer.sh

	// When both iptables modes available, we choose the one with more rules, because the other one normally outputs empty rules.
	nftIptablesModeAvaiable := true
	legacyIptablesModeAvaiable := true
	legacySaveLineNum := 0
	nftSaveLineNum := 0
	if _, err := exec.LookPath("iptables-legacy-save"); err != nil {
		legacyIptablesModeAvaiable = false
		logger.Info("iptables-legacy-save is not available", zap.Error(err))
	} else {
		legacySaveOut, err := exec.Command("iptables-legacy-save").CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("failed to run iptables-legacy-save, %s", err.Error()) //nolint:goerr113 //no specific handling expected
		}
		legacySaveLineNum = len(strings.Split(string(legacySaveOut), "\n"))
	}

	if _, err := exec.LookPath("iptables-nft-save"); err != nil {
		nftIptablesModeAvaiable = false
		logger.Info("iptables-nft-save is not available", zap.Error(err))
	} else {
		nftSaveOut, err := exec.Command("iptables-nft-save").CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("failed to run iptables-nft-save, %s", err.Error()) //nolint:goerr113 //no specific handling expected
		}
		nftSaveLineNum = len(strings.Split(string(nftSaveOut), "\n"))
	}

	if nftIptablesModeAvaiable && legacyIptablesModeAvaiable {
		if legacySaveLineNum > nftSaveLineNum {
			return legacyIptablesMode, nil
		}
		return nftIptablesMode, nil
	}

	if nftIptablesModeAvaiable {
		return nftIptablesMode, nil
	}

	if legacyIptablesModeAvaiable {
		return legacyIptablesMode, nil
	}

	return "", errIptablesUnavilable
}
