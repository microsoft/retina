// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package outputlocation

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"go.uber.org/zap"

	captureConstants "github.com/microsoft/retina/pkg/capture/constants"
	"github.com/microsoft/retina/pkg/log"
)

type PersistentVolumeClaim struct {
	l *log.ZapLogger
}

var _ Location = &PersistentVolumeClaim{}

func NewPersistentVolumeClaim(logger *log.ZapLogger) Location {
	return &PersistentVolumeClaim{l: logger}
}

func (pvc *PersistentVolumeClaim) Name() string {
	return "PersistentVolumeClaim"
}

func (pvc *PersistentVolumeClaim) Enabled() bool {
	pvcOutput := os.Getenv(string(captureConstants.CaptureOutputLocationEnvKeyPersistentVolumeClaim))
	if len(pvcOutput) == 0 {
		pvc.l.Debug("Output location is not enabled", zap.String("location", pvc.Name()))
		return false
	}
	return true
}

func (pvc *PersistentVolumeClaim) Output(_ context.Context, srcFilePath string) error {
	dstDir := captureConstants.PersistentVolumeClaimVolumeMountPathLinux
	pvc.l.Info("Copy file",
		zap.String("location", pvc.Name()),
		zap.String("source file path", srcFilePath),
		zap.String("destination file path", dstDir),
	)
	srcFile, err := os.Open(srcFilePath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	fileName := filepath.Base(srcFilePath)
	fileHostPath := filepath.Join(dstDir, fileName)
	destFile, err := os.Create(fileHostPath)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile)
	return err
}
