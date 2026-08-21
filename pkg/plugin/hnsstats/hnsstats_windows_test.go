// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package hnsstats

import (
	"context"
	"errors"
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

var errRegistryFailure = errors.New("registry failure")

// TestStart_CiliumRegistryMatrix asserts hnsstats only collects stats when Cilium on
// Windows is disabled. It is the complement of the equivalent ebpfwindows matrix: for
// every registry state exactly one of the two plugins may start.
//
// The context is cancelled up front, so when hnsstats does proceed pullHnsStats returns
// immediately via Stop() (leaving state == stop) without querying HNS or VFP. A skipped
// plugin never reaches pullHnsStats, so its state stays at start.
func TestStart_CiliumRegistryMatrix(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())

	tests := []struct {
		name          string
		ciliumEnabled bool
		ciliumErr     error
		wantState     int
		wantErr       error
	}{
		{
			name:      "registry value missing or 0 starts hnsstats",
			wantState: stop,
		},
		{
			name:          "registry value 1 skips hnsstats in favour of ebpfwindows",
			ciliumEnabled: true,
			wantState:     start,
		},
		{
			name:      "registry read error fails hnsstats",
			ciliumErr: errRegistryFailure,
			wantState: start,
			wantErr:   errRegistryFailure,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalCheck := isCiliumOnWindowsEnabled
			t.Cleanup(func() { isCiliumOnWindowsEnabled = originalCheck })

			isCiliumOnWindowsEnabled = func() (bool, error) { return tt.ciliumEnabled, tt.ciliumErr }

			p := &hnsstats{
				cfg: &kcfg.Config{MetricsInterval: 100 * time.Second},
				l:   log.Logger().Named(name),
			}

			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			err := p.Start(ctx)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.wantState, p.state, "hnsstats collection start decision")
		})
	}
}
