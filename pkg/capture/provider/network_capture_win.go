//go:build windows
// +build windows

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package provider

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"go.uber.org/zap"

	captureConstants "github.com/microsoft/retina/pkg/capture/constants"
	"github.com/microsoft/retina/pkg/log"
)

type NetworkCaptureProvider struct {
	NetworkCaptureProviderCommon
	TmpCaptureDir string
	CaptureName   string
	NodeHostName  string

	l *log.ZapLogger
}

var _ NetworkCaptureProviderInterface = &NetworkCaptureProvider{}

func NewNetworkCaptureProvider(logger *log.ZapLogger) NetworkCaptureProviderInterface {
	return &NetworkCaptureProvider{
		NetworkCaptureProviderCommon: NetworkCaptureProviderCommon{l: logger},
		l:                            logger,
	}
}

func (ncp *NetworkCaptureProvider) Setup(captureName, nodeHostname string) (string, error) {
	captureFolderDir, err := ncp.NetworkCaptureProviderCommon.Setup(captureName, nodeHostname)
	if err != nil {
		return "", err
	}
	ncp.l.Info("Created temporary folder for network capture", zap.String("capture temporary folder", captureFolderDir))

	ncp.TmpCaptureDir = captureFolderDir
	ncp.CaptureName = captureName
	ncp.NodeHostName = nodeHostname
	return ncp.TmpCaptureDir, nil
}

func (ncp *NetworkCaptureProvider) CaptureNetworkPacket(filter string, duration, maxSizeMB int, sigChan <-chan os.Signal) error {
	stopTrace, err := ncp.needToStopTraceSession()
	if err != nil {
		return err
	}
	if stopTrace {
		ncp.l.Info("Stopping netsh trace session before starting a new one")
		_ = ncp.stopNetworkCapture()
	}

	captureFileName := ncp.NetworkCaptureProviderCommon.CaptureNodetimestampName(ncp.CaptureName, ncp.NodeHostName)
	captureFileName = captureFileName + ".etl"
	captureFilePath := filepath.Join(ncp.TmpCaptureDir, captureFileName)

	captureStartCmd := exec.Command(
		"cmd", "/C",
		fmt.Sprintf("netsh trace start capture=yes report=disabled overwrite=yes"),
		fmt.Sprintf("tracefile=%s", captureFilePath),
	)

	// We should split arguments organized in a string delimited by spaces as
	// seperate ones, otherwise the whole string will be treated as one argument.
	// For example, given the following filter, exec lib will treat IPv4.Address
	// as the argument and the rest as the value of IPv4.Address.
	// "IPv4.Address=(10.244.1.85,10.244.1.235) IPv6.Address=(fd5c:d9f1:79c5:fd83::1bc,fd5c:d9f1:79c5:fd83::11b)"
	if len(filter) != 0 {
		netshFilterSlice := strings.Split(filter, " ")
		captureStartCmd.Args = append(captureStartCmd.Args, netshFilterSlice...)
	}

	// NOTE: netsh cannot stop when the given max size of reach reaches, but we can use maxSizeMB to limit the size of
	// trace file.
	if maxSizeMB != 0 {
		captureStartCmd.Args = append(captureStartCmd.Args, fmt.Sprintf("maxSize=%d", maxSizeMB))
	}
	ncp.l.Info("Running netsh with args", zap.String("netsh command", captureStartCmd.String()), zap.Any("netsh args", captureStartCmd.Args))

	netshLogFile, err := ncp.NetworkCaptureProviderCommon.networkCaptureCommandLog("netsh.log", captureStartCmd)
	if err != nil {
		return err
	}

	defer func() {
		if netshLog, err := os.ReadFile(netshLogFile.Name()); err != nil {
			ncp.l.Warn("Failed to read netsh log", zap.Error(err))
		} else {
			ncp.l.Info(fmt.Sprintf("Netsh command output: %s", string(netshLog)))
		}
		netshLogFile.Close()
	}()

	err = captureStartCmd.Start()
	if err != nil {
		ncp.l.Error("Failed to start netsh", zap.Error(err))
		return err
	}

	doneChan := make(chan bool, 1)
	if duration != 0 {
		go func() {
			time.Sleep(time.Second * time.Duration(duration))
			doneChan <- true
		}()
	}

	select {
	case <-doneChan:
	case sig := <-sigChan:
		ncp.l.Info("Got OS signal, netsh will be stopped", zap.String("signal", sig.String()))
	}

	ncp.l.Info("Stop netsh")
	if err := ncp.stopNetworkCapture(); err != nil {
		ncp.l.Error("Failed to stop netsh trace by 'netsh trace stop', will kill the process", zap.Error(err))
		_ = captureStartCmd.Process.Kill()
		return fmt.Errorf("netsh stop failed: Output: %s", err)
	}

	if err := etl2pcapng(captureFilePath); err != nil {
		return err
	}

	return nil
}

// needToStopTraceSession returns true when a running trace session started by Retina capture exists, otherwise returns
// false. Specially, when the trace session is not started by Retina capture, determined from the capture file path, an
// error will be raised.
func (ncp *NetworkCaptureProvider) needToStopTraceSession() (bool, error) {
	command := exec.Command("cmd", "/C", "netsh trace show status")
	output, err := command.CombinedOutput()

	// When there's no running trace session, `netsh trace show status` will exist with error code 1, in which case we
	// should not raise error.
	if strings.Contains(string(output), "There is no trace session currently in progress") {
		ncp.l.Info("There is no running trace session")
		return false, nil
	}

	if err != nil {
		ncp.l.Error("Failed to get netsh trace status", zap.String("command", command.String()), zap.String("command output", string(output)), zap.Error(err))
		return false, err
	}

	if strings.Contains(string(output), captureLabelFolderName) {
		ncp.l.Info("There is a running trace session created by Retina capture")
		return true, nil
	}
	ncp.l.Info("There is a running trace session NOT created by Retina capture", zap.String("command output", string(output)))
	return false, fmt.Errorf("cannot stop trace session because it's not created by Retina capture")
}

func (ncp *NetworkCaptureProvider) stopNetworkCapture() error {
	ncp.l.Info("Stopping netsh trace session")

	command := exec.Command("cmd", "/C", "netsh trace stop")
	output, err := command.CombinedOutput()
	// ignore the error when stop the trace when no live trace session exists.
	if strings.Contains(string(output), "There is no trace session currently in progress") {
		return nil
	}
	if err != nil {
		ncp.l.Error("Failed to stop netsh trace by 'netsh trace stop'", zap.Error(err), zap.String("command output", string(output)))
		return err
	}
	ncp.l.Info("Done for stopping netsh trace session")
	return nil
}

func etl2pcapng(etlCaptureFilePath string) error {
	captureFileDir := filepath.Dir(etlCaptureFilePath)
	captureFilePathWithoutExt := strings.TrimSuffix(filepath.Base(etlCaptureFilePath), filepath.Ext(etlCaptureFilePath))
	pcapCaptureFileName := captureFilePathWithoutExt + ".pcap"
	pcapCaptureFilePath := filepath.Join(captureFileDir, pcapCaptureFileName)
	containerSandboxMountPoint := os.Getenv(captureConstants.ContainerSandboxMountPointEnvKey)
	if len(containerSandboxMountPoint) == 0 {
		return fmt.Errorf("failed to find sandbox mount path through env %s", captureConstants.ContainerSandboxMountPointEnvKey)
	}
	etl2pcapngBinPath := filepath.Join(containerSandboxMountPoint, "etl2pcapng.exe")
	etl2pcapngCmd := exec.Command(
		"cmd", "/C",
		fmt.Sprintf("%s %s %s", etl2pcapngBinPath, etlCaptureFilePath, pcapCaptureFilePath),
	)
	if output, err := etl2pcapngCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("etl2pcapng failed to convert %s to pcap, Cmd: %s, Error: %s, Output: %s", etlCaptureFilePath, etl2pcapngCmd.String(), err, string(output))
	}
	// Keep etl trace file as well to enrich networking debugging info.
	return nil
}

func (ncp *NetworkCaptureProvider) CollectMetadata() error {
	ncp.l.Info("Start to collect network metadata")

	// Download collectlogs.ps1 if not exists.
	// Platforms like Azure Kubernetes Service caches collectlogs.ps1 for debug and it's beneficial for air-gapped
	// environment in the hard-coded path `c:\\k\\debug` as required by the script.
	// ref: https://github.com/Azure/AgentBaker/tree/master/staging/cse/windows/debug
	collectLogFilePath := "c:\\k\\debug\\collectlogs.ps1"
	if _, err := os.Stat(collectLogFilePath); os.IsNotExist(err) {
		// TODO(mainred): Should we delete collectlogs.ps1 if we download it? Besides, collectlogs.ps1 will download a
		// series of tool scripts under folder c:\\k\\debug.
		out, err := os.Create(collectLogFilePath)
		if err != nil {
			return err
		}
		defer out.Close()

		// NOTE(mainred): Scripts downloaded by collectlogs.ps1 always use master branch, so we here use master branch
		// as well to keep version compatible. And unfortunately, there's no tag/release in microsoft/SDN for us to
		// pin a specific version.
		url := "https://raw.githubusercontent.com/microsoft/SDN/master/Kubernetes/windows/debug/collectlogs.ps1"
		resp, err := http.Get(url)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		// Write the body to file
		_, err = io.Copy(out, resp.Body)
		if err != nil {
			return err
		}
	}

	// NOTE(mainred): Windows image built lost the windows powershell in PATH environment variable, tracked by https://github.com/microsoft/retina/issues/307
	// Before the issue is resolved, we hard-code powershell path as a workaround for this issue. This workaround will
	// not work if powershell path changes in the future.
	pathEnv := os.Getenv("PATH")
	powershellCommand := "powershell"
	powershellPath := "C:\\WINDOWS\\System32\\WindowsPowerShell\\v1.0"
	if !strings.Contains(pathEnv, powershellPath) {
		pathEnvWithPowershell := fmt.Sprintf("%s;%s", pathEnv, powershellPath)
		if err := os.Setenv("PATH", pathEnvWithPowershell); err != nil {
			ncp.l.Warn("Failed to set PATH environment variable", zap.Error(err))
			powershellCommand = powershellPath + "\\powershell"
		}
	}

	out, err := exec.Command(powershellCommand, "-file", collectLogFilePath).CombinedOutput()
	if err != nil {
		ncp.l.Error("Failed to collect windows metadata through collectlogs.ps1", zap.String("collect log script path", collectLogFilePath), zap.String("output", string(out)))
		return err
	}

	ncp.l.Info("Succeeded in running collectlogs.ps1", zap.String("output", string(out)))

	// Normally, the log path will be printed at the end of the output of collectlogs.ps1, with the following pattern.
	// Logs are available at C:\k\debug\y5iyszva.4o4\n\r\n\r\n
	re := regexp.MustCompile(`Logs are available at ([^\r\n]+)`)
	matches := re.FindStringSubmatch(string(out))
	if len(matches) == 0 {
		return fmt.Errorf("failed to get diagnostic log file")
	}
	logFilePath := matches[1]
	logFilePath = strings.Trim(logFilePath, " ")
	defer os.RemoveAll(logFilePath) //nolint:errcheck

	// Create a separate folder for the long list of metadata files.
	captureMetadataFolderDir := filepath.Join(ncp.TmpCaptureDir, "metadata")
	if err := os.Mkdir(captureMetadataFolderDir, 0o750); err != nil {
		return err
	}

	if output, err := exec.Command(
		"cmd", "/C",
		"xcopy",
		logFilePath,
		captureMetadataFolderDir,
		"/e", // `/e` copies directories and subdirectories, including empty ones.
	).CombinedOutput(); err != nil {
		ncp.l.Error("Failed to copy log file", zap.String("output", string(output)))
		return err
	}

	ncp.l.Info("Done for collecting network metadata")
	return nil
}

func (ncp *NetworkCaptureProvider) Cleanup() error {
	ncp.l.Info("Cleanup network capture", zap.String("capture name", ncp.CaptureName), zap.String("temporary dir", ncp.TmpCaptureDir))
	ncp.NetworkCaptureProviderCommon.Cleanup()
	return nil
}
