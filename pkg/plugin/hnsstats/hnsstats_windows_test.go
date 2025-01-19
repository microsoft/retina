// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package hnsstats

import (
	"context"
	"testing"
	"time"

	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/log"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

func TestShutdown(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	p := &hnsstats{
		cfg: &kcfg.Config{
			MetricsInterval: 100 * time.Second,
			EnablePodLevel:  true,
		},
		l: log.Logger().Named(name),
	}
	p.Init()
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
