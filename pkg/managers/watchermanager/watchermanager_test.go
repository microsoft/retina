// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package watchermanager

import (
	"context"
	"testing"

	"github.com/microsoft/retina/pkg/log"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"golang.org/x/sync/errgroup"
)

func TestStopWatcherManagerGracefully(t *testing.T) {
	ctl := gomock.NewController(t)
	defer ctl.Finish()
	log.SetupZapLogger(log.GetDefaultLogOpts())
	mgr := NewWatcherManager()

	ctx := context.Background()
	g, errctx := errgroup.WithContext(ctx)

	var err error
	g.Go(func() error {
		err = mgr.Start(errctx)
		return err
	})
	mgr.Stop(errctx)
	require.NoError(t, err)
}

func TestWatcherStopWithoutStart(t *testing.T) {
	ctl := gomock.NewController(t)
	defer ctl.Finish()
	log.SetupZapLogger(log.GetDefaultLogOpts())

	mgr := NewWatcherManager()

	err := mgr.Stop(context.Background())
	require.Nil(t, err, "Expected no error when stopping watcher manager without starting it")
}
