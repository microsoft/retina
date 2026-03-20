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
	Cfg *E2EConfig
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
	*l.Cfg = *cfg
	l.Cfg.Paths = *ResolvePaths(filepath.Dir(filepath.Dir(cwd)))

	if l.Cfg.Image.Tag == "" {
		tag, err := DevTag(l.Cfg.Paths.RootDir)
		if err != nil {
			return fmt.Errorf("generate dev tag: %w", err)
		}
		l.Cfg.Image.Tag = tag
		log.Printf("no TAG provided, will build images as %s", tag)
	}
	return nil
}
