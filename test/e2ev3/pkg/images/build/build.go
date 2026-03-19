// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package build

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

// Images builds Retina container images by invoking the top-level Makefile.
// It builds the agent, init, and operator images for linux/amd64.
type Images struct {
	RootDir   string
	Registry  string
	Namespace string
	Tag       string
	Push      bool // push to registry after build (for Azure); if false, loads into local docker daemon (for Kind)
}

func (b *Images) Do(ctx context.Context) error {
	targets := []string{"retina-image", "retina-operator-image"}

	errs := make(chan error, len(targets))
	for _, target := range targets {
		go func(t string) {
			errs <- b.runMake(ctx, t)
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

func (b *Images) runMake(ctx context.Context, target string) error {
	args := []string{
		target,
		"PLATFORM=linux/amd64",
		"TAG=" + b.Tag,
		"RETINA_PLATFORM_TAG=" + b.Tag, // use base tag so image refs match Helm values
		"IMAGE_REGISTRY=" + b.Registry,
		"IMAGE_NAMESPACE=" + b.Namespace,
	}
	if b.Push {
		args = append(args, "BUILDX_ACTION=--push", "OUTPUT_LOCAL=")
	} else {
		// Load into local docker daemon for Kind sideloading.
		// Disable provenance/sbom attestations — Kind's ctr import can't handle them.
		args = append(args, "BUILDX_ACTION=--load --provenance=false --sbom=false", "OUTPUT_LOCAL=")
	}

	log.Printf("building: make %s", strings.Join(args, " "))

	cmd := exec.CommandContext(ctx, "make", args...)
	cmd.Dir = b.RootDir

	// Stream both stdout and stderr so buildx progress is visible.
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("make %s failed: %w", target, err)
	}
	return nil
}


