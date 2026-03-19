//go:build e2e

package config

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// Step resolves e2e config, paths, and image tag.
type Step struct {
	Params *E2EParams
}

func (l *Step) String() string { return "load-config" }

func (l *Step) Do(_ context.Context) error {
	cfg, err := LoadE2EConfig()
	if err != nil {
		return fmt.Errorf("load e2e config: %w", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get cwd: %w", err)
	}
	l.Params.Cfg = cfg
	l.Params.Paths = ResolvePaths(filepath.Dir(filepath.Dir(cwd)))

	if cfg.Image.Tag == "" {
		tag, err := DevTag(l.Params.Paths.RootDir)
		if err != nil {
			return fmt.Errorf("generate dev tag: %w", err)
		}
		cfg.Image.Tag = tag
		log.Printf("no TAG provided, will build images as %s", tag)
	}
	return nil
}
