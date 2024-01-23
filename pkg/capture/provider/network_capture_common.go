// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package provider

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/microsoft/retina/pkg/log"
	"go.uber.org/zap"
)

// captureLabelFolderName is the folder name to label the path as created by Kapppie.
const captureLabelFolderName = "retina-capture" //nolint:unused

// captureNodetimestampName returns a unique name with the current UTC timestamp and provided variables.
func (ncpc *NetworkCaptureProviderCommon) CaptureNodetimestampName(captureName, nodeHostname string) string {
	formatedUTCTime := strings.Replace(time.Now().UTC().Format("2006#01#02#03#04#05UTC"), "#", "", -1)
	uniqueName := fmt.Sprintf("%s-%s-%s", captureName, nodeHostname, formatedUTCTime)
	return uniqueName
}

type NetworkCaptureProviderCommon struct {
	TmpCaptureDir string
	l             *log.ZapLogger
}

func (ncpc *NetworkCaptureProviderCommon) Setup(captureName, nodeHostname string) (string, error) {
	captureFolderName := ncpc.CaptureNodetimestampName(captureName, nodeHostname)
	captureFolderDir := filepath.Join(os.TempDir(), captureFolderName)
	err := os.MkdirAll(captureFolderDir, 0o750)
	if err != nil {
		return "", err
	}
	ncpc.TmpCaptureDir = captureFolderDir
	return ncpc.TmpCaptureDir, nil
}

func (ncpc *NetworkCaptureProviderCommon) Cleanup() {
	if err := os.RemoveAll(ncpc.TmpCaptureDir); err != nil {
		ncpc.l.Error("Failed to delete folder", zap.String("folder name", ncpc.TmpCaptureDir), zap.Error(err))
	}
}

func (ncpc *NetworkCaptureProviderCommon) networkCaptureCommandLog(logFileName string, captureCommand *exec.Cmd) (*os.File, error) {
	captureCommandLogFilePath := filepath.Join(ncpc.TmpCaptureDir, logFileName)
	captureCommandLogFile, err := os.OpenFile(captureCommandLogFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		ncpc.l.Error("Failed to create network capture command log file", zap.String("network capture command log file path", captureCommandLogFilePath), zap.Error(err))
		return nil, err
	}

	if _, err := captureCommandLogFile.WriteString(fmt.Sprintf("%s\n\n", captureCommand.String())); err != nil {
		ncpc.l.Error("Failed to write capture command to file", zap.String("file", captureCommandLogFile.Name()), zap.Error(err))
	}

	captureCommand.Stdout = captureCommandLogFile
	captureCommand.Stderr = captureCommandLogFile
	return captureCommandLogFile, nil
}
