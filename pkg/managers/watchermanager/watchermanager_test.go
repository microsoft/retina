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

var errInitFailed = errors.New("init failed")

func TestStopWatcherManagerGracefully(t *testing.T) {
	ctl := gomock.NewController(t)
	defer ctl.Finish()
	log.SetupZapLogger(log.GetDefaultLogOpts())
	mgr := NewWatcherManager()

	mockAPIServerWatcher := mock.NewMockIWatcher(ctl)
	mockEndpointWatcher := mock.NewMockIWatcher(ctl)

	mgr.Watchers = []IWatcher{
		mockEndpointWatcher,
		mockAPIServerWatcher,
	}

	mockAPIServerWatcher.EXPECT().Init(gomock.Any()).Return(nil).AnyTimes()
	mockEndpointWatcher.EXPECT().Init(gomock.Any()).Return(nil).AnyTimes()

	mockEndpointWatcher.EXPECT().Stop(gomock.Any()).Return(nil).AnyTimes()
	mockAPIServerWatcher.EXPECT().Stop(gomock.Any()).Return(nil).AnyTimes()

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

	mockAPIServerWatcher := mock.NewMockIWatcher(ctl)
	mockEndpointWatcher := mock.NewMockIWatcher(ctl)

	mgr := NewWatcherManager()
	mgr.Watchers = []IWatcher{
		mockAPIServerWatcher,
		mockEndpointWatcher,
	}

	mockAPIServerWatcher.EXPECT().Init(gomock.Any()).Return(errInitFailed).AnyTimes()
	mockEndpointWatcher.EXPECT().Init(gomock.Any()).Return(errInitFailed).AnyTimes()

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
