// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package log

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestLogFileRotation(t *testing.T) {
	lOpts := &LogOpts{
		Level:         "info",
		File:          true,
		FileName:      "test.log",
		MaxFileSizeMB: 1,
		MaxBackups:    3,
		MaxAgeDays:    1,
	}

	SetupZapLogger(lOpts)

	logsToPrint := 10
	for i := 0; i < logsToPrint; i++ {
		global.Info("test", zap.Int("i", i))
	}
	global.Close()

	_, err := os.Stat(lOpts.FileName)
	assert.NoError(t, err, "Test log file is not found")

	// change this to 4 if using logsToPrint as 100000
	expectedReplicas := 1
	curReplicas := 0
	p, err := os.Getwd()
	assert.NoError(t, err, "Getwd failed with err")

	err = filepath.Walk(p, func(path string, info os.FileInfo, err error) error {
		global.Info("Filename: ", zap.String("path", path), zap.String("name", info.Name()))
		if !info.IsDir() {
			if strings.HasPrefix(info.Name(), "test") && strings.HasSuffix(info.Name(), ".log") {
				curReplicas++
			}
		}
		return nil
	})
	assert.NoError(t, err, "Test log file walk through failed with err")
	assert.Equal(t, expectedReplicas, curReplicas, "Test log file replicas are not as expected on 2nd try")
}
