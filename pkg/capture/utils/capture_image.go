// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package utils

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

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

var (
	ErrInvalidImageFormat  = errors.New("invalid image name format")
	ErrMCRAPIRequestFailed = errors.New("MCR API request failed")
	ErrNoVersionTagsFound  = errors.New("no version tags found in MCR")
)

type VersionSource string

const (
	// VersionSourceCLI defines the version source as CLI version.
	VersionSourceCLIVersion VersionSource = "CLI version"
	// VersionSourceImage defines the version source as image version.
	VersionSourceOperatorImageVersion VersionSource = "Operator Image"
)

// getMostRecentMCRTag fetches the most recent version tag from MCR for the retina-agent
func getMostRecentMCRTag(imageName string) (string, error) {
	// Extract repository path from image name (e.g., "mcr.microsoft.com/containernetworking/retina-agent")
	parts := strings.SplitN(imageName, "/", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("%w: %s", ErrInvalidImageFormat, imageName)
	}

	repo := parts[1]
	url := fmt.Sprintf("https://mcr.microsoft.com/v2/%s/tags/list", repo)

	// Create request with context and timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch tags from MCR: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w with status code %d", ErrMCRAPIRequestFailed, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	var result struct {
		Tags []string `json:"tags"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// Find the most recent version tag by iterating in reverse (tags are typically sorted)
	// Look for tags that start with 'v' and don't contain platform suffixes
	for i := len(result.Tags) - 1; i >= 0; i-- {
		tag := result.Tags[i]
		if strings.HasPrefix(tag, "v") && !strings.Contains(tag, "-linux") && !strings.Contains(tag, "-windows") {
			return tag, nil
		}
	}

	return "", ErrNoVersionTagsFound
}

// CaptureWorkloadImage returns the container image to use for capture workload jobs.
// For MCR images without a tag, it automatically fetches the most recent version tag from the registry.
// If a tag is already specified, it uses that tag directly.
// Otherwise, it uses the CLI version or allows override via environment variable for testing.
func CaptureWorkloadImage(logger *log.ZapLogger, imageVersion string, debug bool, vs VersionSource) string {
	defaultCaptureWorkloadImageVersion := imageVersion
	defaultCaptureWorkloadImageName := captureConstants.CaptureWorkloadImageName

	// If the image is from MCR and doesn't already have a tag, fetch the most recent tag
	if strings.HasPrefix(defaultCaptureWorkloadImageName, "mcr.microsoft.com/") {
		// Check if a tag is already specified
		if strings.Contains(defaultCaptureWorkloadImageName, ":") {
			logger.Info(fmt.Sprintf("Using MCR capture workload image %s with specified tag", defaultCaptureWorkloadImageName))
			return defaultCaptureWorkloadImageName
		}

		// No tag specified, fetch the most recent one
		latestTag, err := getMostRecentMCRTag(defaultCaptureWorkloadImageName)
		if err == nil {
			captureWorkloadImage := defaultCaptureWorkloadImageName + ":" + latestTag
			logger.Info(fmt.Sprintf("Using MCR capture workload image %s with latest tag from MCR registry", captureWorkloadImage))
			return captureWorkloadImage
		}
		logger.Warn("Failed to fetch latest MCR tag, falling back to CLI version", zap.Error(err))
	}

	// For testing.
	if debug {
		captureWorkloadImageFromEnv := os.Getenv(captureWorkloadImageEnvKey)
		if captureWorkloadImageFromEnv != "" {
			logger.Info("Debug mode: obtained capture workload image from environment variable", zap.String("environment variable key", captureWorkloadImageEnvKey), zap.String("image", captureWorkloadImageFromEnv))
			return captureWorkloadImageFromEnv
		}

		debugCaptureWorkloadImageName := captureConstants.DebugCaptureWorkloadImageName
		debugCaptureWorkloadImage := debugCaptureWorkloadImageName + ":" + defaultCaptureWorkloadImageVersion
		logger.Info(fmt.Sprintf("Debug mode: Using capture workload image %s with version determined by %s", debugCaptureWorkloadImage, vs))
		return debugCaptureWorkloadImage
	}

	captureWorkloadImage := defaultCaptureWorkloadImageName + ":" + defaultCaptureWorkloadImageVersion
	logger.Info(fmt.Sprintf("Using capture workload image %s with version determined by %s", captureWorkloadImage, vs))

	return captureWorkloadImage
}
