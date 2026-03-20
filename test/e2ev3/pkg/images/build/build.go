// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package build

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

	"github.com/microsoft/retina/test/e2ev3/config"
	"github.com/microsoft/retina/test/e2ev3/pkg/images"
	"github.com/microsoft/retina/test/e2ev3/pkg/utils"
)

// Step builds Retina container images by invoking the top-level Makefile.
// It builds the agent, init, and operator images for linux/amd64.
// If all images already exist locally and ForceBuild is false, the build is skipped.
type Step struct {
	Cfg *config.E2EConfig
}

func (b *Step) String() string { return "build-images" }

func (b *Step) Do(ctx context.Context) error {
	img := &b.Cfg.Image
	if !*config.ForceBuild && allImagesExist(img.Registry, img.Namespace, img.Tag) {
		slog.Info("all images already present locally, skipping build")
		return nil
	}

	push := *config.Provider != "kind"
	return b.build(ctx, b.Cfg.Paths.RootDir, img.Registry, img.Namespace, img.Tag, push)
}

func (b *Step) build(ctx context.Context, rootDir, registry, namespace, tag string, push bool) error {
	targets := []string{"retina-image", "retina-operator-image"}

	errs := make(chan error, len(targets))
	for _, target := range targets {
		go func(t string) {
			errs <- runMake(ctx, rootDir, registry, namespace, tag, push, t)
		}(target)
	}

	var firstErr error
	for range targets {
		if err := <-errs; err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func runMake(ctx context.Context, rootDir, registry, namespace, tag string, push bool, target string) error {
	args := []string{
		target,
		"PLATFORM=linux/amd64",
		"TAG=" + tag,
		"RETINA_PLATFORM_TAG=" + tag,
		"IMAGE_REGISTRY=" + registry,
		"IMAGE_NAMESPACE=" + namespace,
	}
	if push {
		args = append(args, "BUILDX_ACTION=--push", "OUTPUT_LOCAL=")
	} else {
		// Load into local docker daemon for Kind sideloading.
		// Disable provenance/sbom attestations — Kind's ctr import can't handle them.
		args = append(args, "BUILDX_ACTION=--load --provenance=false --sbom=false", "OUTPUT_LOCAL=")
	}

	slog.Info("building image", "command", "make "+strings.Join(args, " "))

	cmd := exec.CommandContext(ctx, "make", args...)
	cmd.Dir = rootDir
	cmdOut := &utils.SlogWriter{Level: slog.LevelInfo, Source: "make-" + target}
	cmd.Stdout = cmdOut
	cmd.Stderr = cmdOut

	if err := cmd.Run(); err != nil {
		cmdOut.Flush()
		return fmt.Errorf("make %s failed: %w", target, err)
	}
	cmdOut.Flush()
	return nil
}

// allImagesExist returns true if every Retina image is already in the local Docker daemon.
func allImagesExist(registry, namespace, tag string) bool {
	for _, ref := range images.RetinaImages(registry, namespace, tag) {
		cmd := exec.Command("docker", "image", "inspect", ref)
		if err := cmd.Run(); err != nil {
			return false
		}
	}
	return true
}

