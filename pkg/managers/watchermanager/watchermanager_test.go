// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package watchermanager

import (
	"context"
	"errors"
	"testing"

	"github.com/microsoft/retina/pkg/log"
	mock "github.com/microsoft/retina/pkg/managers/watchermanager/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"golang.org/x/sync/errgroup"
)

func TestStopWatcherManagerGracefully(t *testing.T) {
	ctl := gomock.NewController(t)
	defer ctl.Finish()
	log.SetupZapLogger(log.GetDefaultLogOpts())
	mgr := NewWatcherManager()

	ctx, _ := context.WithCancel(context.Background())
	g, errctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return mgr.Start(errctx)
	})
	err := g.Wait()

	mgr.Stop(errctx)
	require.NoError(t, err)
}

func TestWatcherInitFailsGracefully(t *testing.T) {
	ctl := gomock.NewController(t)
	defer ctl.Finish()
	log.SetupZapLogger(log.GetDefaultLogOpts())

	mockApiServerWatcher := mock.NewMockIWatcher(ctl)
	mockEndpointWatcher := mock.NewMockIWatcher(ctl)

	mgr := NewWatcherManager()
	mgr.Watchers = []IWatcher{
		mockApiServerWatcher,
		mockEndpointWatcher,
	}

	mockApiServerWatcher.EXPECT().Init(gomock.Any()).Return(errors.New("error")).AnyTimes()
	mockEndpointWatcher.EXPECT().Init(gomock.Any()).Return(errors.New("error")).AnyTimes()

	err := mgr.Start(context.Background())
	require.NotNil(t, err, "Expected error when starting watcher manager")
}

func TestWatcherStopWithoutStart(t *testing.T) {
	ctl := gomock.NewController(t)
	defer ctl.Finish()
	log.SetupZapLogger(log.GetDefaultLogOpts())

	mgr := NewWatcherManager()

	err := mgr.Stop(context.Background())
	require.Nil(t, err, "Expected no error when stopping watcher manager without starting it")
}
