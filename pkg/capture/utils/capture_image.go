// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package utils

import (
	"fmt"
	"os"
	"strings"

	"go.uber.org/zap"

	captureConstants "github.com/microsoft/retina/pkg/capture/constants"
	"github.com/microsoft/retina/pkg/log"
)

const (
	// captureWorkloadImageEnvKey defines the environment variable for retina-agent(capture workload) image.
	// Normally, retina-agent image version is determined by the CLI version but we allow specifying the image through
	// environment variables for testing.
	captureWorkloadImageEnvKey string = "RETINA_AGENT_IMAGE"

	DebugModeEnvKey string = "DEBUG"
)

type VersionSource string

const (
	// VersionSourceCLI defines the version source as CLI version.
	VersionSourceCLIVersion VersionSource = "CLI version"
	// VersionSourceImage defines the version source as image version.
	VersionSourceOperatorImageVersion VersionSource = "Operator Image"
)

// TODO: currently, we return only the default capture workload image for official release in the phase of preview, and
// using the same version for CLI and capture workload image makes sure there's no compatibility issue.
// We can consider exposing the image name and version through CLI flags and adding version compatibility validation for
// CLI and capture workload image.
func CaptureWorkloadImage(logger *log.ZapLogger, imageVersion string, debug bool, vs VersionSource) string {
	defaultCaptureWorkloadImageVersion := imageVersion

	// For testing.
	if debug {
		captureWorkloadImageFromEnv := os.Getenv(captureWorkloadImageEnvKey)
		if captureWorkloadImageFromEnv != "" {
			logger.Info("Debug mode: obtained capture workload image from environment variable", zap.String("environment variable key", captureWorkloadImageEnvKey), zap.String("image", captureWorkloadImageFromEnv))
			return captureWorkloadImageFromEnv
		}

		debugCaptureWorkloadImageName := captureConstants.DebugCaptureWorkloadImageName
		
		// If the debug image name already includes a tag, use it as-is
		if strings.Contains(debugCaptureWorkloadImageName, ":") {
			logger.Info(fmt.Sprintf("Debug mode: Using capture workload image %s with version determined by %s", debugCaptureWorkloadImageName, vs))
			return debugCaptureWorkloadImageName
		}
		
		// Otherwise, append the version tag
		debugCaptureWorkloadImage := debugCaptureWorkloadImageName + ":" + defaultCaptureWorkloadImageVersion
		logger.Info(fmt.Sprintf("Debug mode: Using capture workload image %s with version determined by %s", debugCaptureWorkloadImage, vs))
		return debugCaptureWorkloadImage
	}

	defaultCaptureWorkloadImageName := captureConstants.CaptureWorkloadImageName
	
	// If the image name already includes a tag, use it as-is
	if strings.Contains(defaultCaptureWorkloadImageName, ":") {
		logger.Info(fmt.Sprintf("Using capture workload image %s with version determined by %s", defaultCaptureWorkloadImageName, vs))
		return defaultCaptureWorkloadImageName
	}
	
	// Otherwise, append the version tag
	captureWorkloadImage := defaultCaptureWorkloadImageName + ":" + defaultCaptureWorkloadImageVersion
	logger.Info(fmt.Sprintf("Using capture workload image %s with version determined by %s", captureWorkloadImage, vs))

	return captureWorkloadImage
}
