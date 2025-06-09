//go:build unix

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package provider

import (
	"context"
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

type NetworkCaptureProvider struct {
	NetworkCaptureProviderCommon
	TmpCaptureDir  string
	CaptureName    string
	NodeHostName   string
	StartTimestamp *file.Timestamp

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
	ncp.CaptureName = filename.CaptureName
	ncp.NodeHostName = filename.NodeHostname
	ncp.StartTimestamp = filename.StartTimestamp
	return ncp.TmpCaptureDir, nil
}

func (ncp *NetworkCaptureProvider) CaptureNetworkPacket(ctx context.Context, filter string, duration, maxSizeMB int) error {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(duration)*time.Second)
	defer cancel()

	filename := file.CaptureFilename{CaptureName: ncp.CaptureName, NodeHostname: ncp.NodeHostName, StartTimestamp: ncp.StartTimestamp}
	captureFileName := fmt.Sprintf("%s.pcap", filename)
	captureFilePath := filepath.Join(ncp.TmpCaptureDir, captureFileName)

	// Remove the folder in case it already exists to mislead the file size check.
	os.Remove(captureFilePath) //nolint:errcheck

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

	if packetSize := os.Getenv(captureConstants.PacketSizeEnvKey); len(packetSize) != 0 {
		captureStartCmd.Args = append(
			captureStartCmd.Args,
			"-s", packetSize,
		)
	}

	// If we set flag and value into the arg item of args, the space between flag and value will not treated as part of
	// value, for example, "-i eth0" will be treated as "-i" and " eth0", thus brings a tcpdump unknown interface error.
	if tcpdumpRawFilter := os.Getenv(captureConstants.TcpdumpRawFilterEnvKey); len(tcpdumpRawFilter) != 0 {
		tcpdumpRawFilterSlice := strings.Split(tcpdumpRawFilter, " ")
		captureStartCmd.Args = append(captureStartCmd.Args, tcpdumpRawFilterSlice...)
	}

	if len(filter) != 0 {
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
			ncp.l.Info(fmt.Sprintf("Tcpdump command output: %s", string(tcpdumpLog)))
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
				errChan <- fmt.Errorf("tcpdump command is not constructed with expected arguments")
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
			return fmt.Errorf("tcpdump stop failed, error: %s", killErr)
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
	name        string
	args        []string
	description string
}

func (ncp *NetworkCaptureProvider) CollectMetadata() error {
	ncp.l.Info("Start to collect network metadata")

	iptablesMode := obtainIptablesMode()
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
				},
			},
		},
	}

	for _, metadata := range metadataList {
		if len(metadata.fileName) != 0 {
			captureMetadataFilePath := filepath.Join(ncp.TmpCaptureDir, metadata.fileName)
			outfile, err := os.OpenFile(captureMetadataFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
			if err != nil {
				ncp.l.Error("Failed to create metadata file", zap.String("metadata file path", captureMetadataFilePath), zap.Error(err))
				continue
			}
			defer outfile.Close()

			if _, err := outfile.WriteString("Summary:\n\n"); err != nil {
				ncp.l.Error("Failed to write summary to file", zap.String("file", outfile.Name()), zap.Error(err))
			}

			// Print headlines for all commands in output file.
			cmds := []*exec.Cmd{}
			for _, command := range metadata.commands {
				cmd := exec.Command(command.name, command.args...)
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
				if _, err := outfile.WriteString(fmt.Sprintf("%s\n\n", cmd.String())); err != nil {
					ncp.l.Error("Failed to write string to file", zap.String("file", outfile.Name()), zap.Error(err))
				}

				cmd.Stdout = outfile
				cmd.Stderr = outfile
				if err := cmd.Run(); err != nil {
					// Don't return for error to continue capturing following network metadata.
					ncp.l.Error("Failed to execute command", zap.String("command", cmd.String()), zap.Error(err))
					// Log the error in output file because this error does not stop capture job pod from finishing,
					// and the job can be recycled automatically leaving no info to debug.
					if _, err = outfile.WriteString(fmt.Sprintf("Failed to run %q, error: %s)", cmd.String(), err.Error())); err != nil {
						ncp.l.Error("Failed to write command run failure", zap.String("command", cmd.String()), zap.Error(err))
					}
				}
			}
		} else {
			for _, command := range metadata.commands {
				cmd := exec.Command(command.name, command.args...)
				// Errors will when copying kernel networking configuration for not all files under /proc/sys/net are
				// readable, like '/proc/sys/net/ipv4/route/flush', which doesn't implement the read function.
				if output, err := cmd.CombinedOutput(); err != nil {
					// Don't return for error to continue capturing following network metadata.
					ncp.l.Error("Failed to execute command", zap.String("command", cmd.String()), zap.String("output", string(output)), zap.Error(err))
				}
			}
		}
	}

	ncp.l.Info("Done for collecting network metadata")

	return nil
}

func (ncp *NetworkCaptureProvider) Cleanup() error {
	ncp.l.Info("Cleanup network capture", zap.String("capture name", ncp.CaptureName), zap.String("temporary dir", ncp.TmpCaptureDir))
	ncp.NetworkCaptureProviderCommon.Cleanup()
	return nil
}

func obtainIptablesMode() string {
	// Since iptables v1.8, nf_tables are introduced as an improvement of legacy iptables, but provides the same user
	// interface as legacy iptables through iptables-nft command.
	// based on: https://github.com/kubernetes-sigs/iptables-wrappers/blob/97b01f43a8e8db07840fc4b95e833a37c0d36b12/iptables-wrapper-installer.sh
	legacySaveOut, _ := exec.Command("iptables-legacy-save").CombinedOutput()
	legacySaveLineNum := len(strings.Split(string(legacySaveOut), "\n"))
	nftSaveOut, _ := exec.Command("iptables-nft-save").CombinedOutput()
	nftSaveLineNum := len(strings.Split(string(nftSaveOut), "\n"))
	if legacySaveLineNum > nftSaveLineNum {
		return "legacy"
	}
	return "nft"
}
