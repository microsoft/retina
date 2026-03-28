// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package log

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestSlogHandler(t *testing.T) {
	// Reset the global logger for this test
	global = nil

	_, err := SetupZapLogger(GetDefaultLogOpts())
	require.NoError(t, err)

	handler := SlogHandler()
	require.NotNil(t, handler)

	logger := slog.New(handler)
	// Should not panic
	logger.Info("test message from slog", "key", "value")

	// SetDefaultSlog should make slog.Default() return zap-backed logger
	SetDefaultSlog()
	slog.Default().Info("default slog test", "source", "TestSlogHandler")

	// SlogLogger should return a new logger
	slogLogger := SlogLogger()
	require.NotNil(t, slogLogger)
	slogLogger.Info("from SlogLogger", "test", true)
}

func TestSlogHandlerFallback(t *testing.T) {
	// Reset global to test fallback behavior
	global = nil

	// Without zap setup, SlogHandler should return a fallback text handler
	handler := SlogHandler()
	require.NotNil(t, handler)

	// Should not panic even without zap being setup
	logger := slog.New(handler)
	logger.Info("fallback test message", "key", "value")
}

func TestLogrLogger(t *testing.T) {
	// Reset the global logger for this test
	global = nil

	_, err := SetupZapLogger(GetDefaultLogOpts())
	require.NoError(t, err)

	logrLogger := LogrLogger()
	require.NotNil(t, logrLogger)

	// Should not panic and should log using the same format as Retina's zap logger
	logrLogger.Info("test message from logr", "key", "value")
	logrLogger.V(1).Info("debug message from logr", "level", 1)
}

func TestLogrLoggerFallback(t *testing.T) {
	// Reset global to test fallback behavior
	global = nil

	// Without zap setup, LogrLogger should return a fallback logger
	logrLogger := LogrLogger()
	require.NotNil(t, logrLogger)

	// Should not panic even without zap being setup
	logrLogger.Info("fallback logr test message", "key", "value")
}
