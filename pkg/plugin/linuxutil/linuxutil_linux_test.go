// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build unit
// +build unit

package linuxutil

import (
	"context"
	"testing"
	"time"

	kcfg "github.com/microsoft/retina/pkg/config"

	"github.com/microsoft/retina/pkg/log"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

var (
	cfgPodLevelEnabled = &kcfg.Config{
		MetricsInterval: 1 * time.Second,
		EnablePodLevel:  true,
	}
	cfgPodLevelDisabled = &kcfg.Config{
		MetricsInterval: 1 * time.Second,
		EnablePodLevel:  false,
	}
)

func TestStop(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	p := &linuxUtil{
		cfg: cfgPodLevelEnabled,
		l:   log.Logger().Named(string(Name)),
	}
	err := p.Stop()
	if err != nil {
		t.Fatalf("Expected no error")
	}
	if p.isRunning {
		t.Fatalf("Expected isRunning to be false")
	}

	p.isRunning = true
	err = p.Stop()
	if err != nil {
		t.Fatalf("Expected no error")
	}
	if p.isRunning {
		t.Fatalf("Expected isRunning to be false")
	}
}

func TestShutdown(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	p := &linuxUtil{
		cfg: &kcfg.Config{
			MetricsInterval: 100 * time.Second,
			EnablePodLevel:  true,
		},
		l: log.Logger().Named(string(Name)),
	}

	ctx, cancel := context.WithCancel(context.Background())
	g, errctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return p.Start(errctx)
	})

	time.Sleep(1 * time.Second)
	cancel()
	err := g.Wait()
	require.NoError(t, err)
}
