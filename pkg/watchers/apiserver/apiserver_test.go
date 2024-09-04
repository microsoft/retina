// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
// nolint

package apiserver

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/microsoft/retina/pkg/log"
	filtermanagermocks "github.com/microsoft/retina/pkg/managers/filtermanager"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"k8s.io/client-go/rest"
)

var errDNS = errors.New("DNS error")

func TestGetWatcher(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())

	a := Watcher()
	assert.NotNil(t, a)

	a_again := Watcher()
	assert.Equal(t, a, a_again, "Expected the same veth watcher instance")
}

func TestAPIServerWatcherStop(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockedFilterManager := filtermanagermocks.NewMockIFilterManager(ctrl)

	// When apiserver is already stopped.
	a := &ApiServerWatcher{
		isRunning:     false,
		l:             log.Logger().Named("apiserver-watcher"),
		filterManager: mockedFilterManager,
		restConfig:    getMockConfig(true),
	}
	err := a.Stop(ctx)
	assert.NoError(t, err, "Expected no error when stopping a stopped apiserver watcher")
	assert.Equal(t, false, a.isRunning, "Expected apiserver watcher to be stopped")

	// Start the watcher.
	err = a.Init(ctx)
	assert.NoError(t, err, "Expected no error when starting a stopped apiserver watcher")

	// Stop the watcher.
	err = a.Stop(ctx)
	assert.NoError(t, err, "Expected no error when stopping a running apiserver watcher")
	assert.Equal(t, false, a.isRunning, "Expected apiserver watcher to be stopped")
}

func TestRefresh(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	mockedResolver := NewMockIHostResolver(ctrl)
	mockedFilterManager := filtermanagermocks.NewMockIFilterManager(ctrl)

	a := &ApiServerWatcher{
		l:             log.Logger().Named("apiserver-watcher"),
		hostResolver:  mockedResolver,
		filterManager: mockedFilterManager,
	}

	// Return 2 random IPs for the host everytime LookupHost is called.
	mockedResolver.EXPECT().LookupHost(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, host string) ([]string, error) {
		return []string{randomIP(), randomIP()}, nil
	}).AnyTimes()

	mockedFilterManager.EXPECT().AddIPs(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mockedFilterManager.EXPECT().DeleteIPs(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	a.Refresh(ctx)
	assert.NoError(t, a.Refresh(context.Background()), "Expected no error when refreshing the cache")
}

func TestDiffCache(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockedResolver := NewMockIHostResolver(ctrl)

	old := make(map[string]struct{})
	new := make(map[string]struct{})

	old["192.168.1.1"] = struct{}{}
	old["192.168.1.2"] = struct{}{}
	new["192.168.1.2"] = struct{}{}
	new["192.168.1.3"] = struct{}{}

	a := &ApiServerWatcher{
		l:            log.Logger().Named("apiserver-watcher"),
		hostResolver: mockedResolver,
		current:      old,
		new:          new,
	}

	created, deleted := a.diffCache()
	assert.Equal(t, 1, len(created), "Expected 1 created host")
	assert.Equal(t, 1, len(deleted), "Expected 1 deleted host")
}

func TestRefreshLookUpAlwaysFail(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	mockedResolver := NewMockIHostResolver(ctrl)

	a := &ApiServerWatcher{
		l:            log.Logger().Named("apiserver-watcher"),
		hostResolver: mockedResolver,
	}

	mockedResolver.EXPECT().LookupHost(gomock.Any(), gomock.Any()).Return(nil, errors.New("Error")).AnyTimes()

	a.Refresh(ctx)
	require.Error(t, a.Refresh(context.Background()), "Expected error when refreshing the cache")
}

func TestInitWithIncorrectURL(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	mockedResolver := NewMockIHostResolver(ctrl)
	mockedFilterManager := filtermanagermocks.NewMockIFilterManager(ctrl)

	a := &ApiServerWatcher{
		l:             log.Logger().Named("apiserver-watcher"),
		hostResolver:  mockedResolver,
		restConfig:    getMockConfig(false),
		filterManager: mockedFilterManager,
	}

	mockedResolver.EXPECT().LookupHost(gomock.Any(), gomock.Any()).Return([]string{}, nil).AnyTimes()
	require.Error(t, a.Init(ctx), "Expected error during init")
}

func randomIP() string {
	return fmt.Sprintf("%d.%d.%d.%d", rand.Intn(256), rand.Intn(256), rand.Intn(256), rand.Intn(256))
}

// Mock function to simulate getting a Kubernetes config
func getMockConfig(isCorrect bool) *rest.Config {
	if isCorrect {
		return &rest.Config{
			Host: "https://kubernetes.default.svc.cluster.local:443",
		}
	}
	return &rest.Config{
		Host: "",
	}
}

func TestRefreshFailsFirstFourAttemptsSucceedsOnFifth(t *testing.T) {
	_, err := log.SetupZapLogger(log.GetDefaultLogOpts())
	require.NoError(t, err)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	mockedResolver := NewMockIHostResolver(ctrl)
	mockedFilterManager := filtermanagermocks.NewMockIFilterManager(ctrl)

	a := &ApiServerWatcher{
		l:             log.Logger().Named("apiserver-watcher"),
		hostResolver:  mockedResolver,
		filterManager: mockedFilterManager,
	}

	// Simulate LookupHost failing the first four times and succeeding on the fifth.
	gomock.InOrder(
		mockedResolver.EXPECT().LookupHost(gomock.Any(), gomock.Any()).Return(nil, errDNS).Times(4),
		mockedResolver.EXPECT().LookupHost(gomock.Any(), gomock.Any()).Return([]string{"127.0.0.1"}, nil).Times(1),
	)

	mockedFilterManager.EXPECT().AddIPs(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mockedFilterManager.EXPECT().DeleteIPs(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	err = a.Refresh(ctx)
	require.NoError(t, err, "Expected no error when refreshing the cache")
}
