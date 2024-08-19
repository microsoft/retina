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
	"github.com/microsoft/retina/pkg/watchers/apiserver/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"k8s.io/client-go/rest"
)

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

	mockedResolver := mocks.NewMockIHostResolver(ctrl)
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

	mockedResolver := mocks.NewMockIHostResolver(ctrl)

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

func TestNoRefreshErrorOnLookupHost(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	mockedResolver := mocks.NewMockIHostResolver(ctrl)

	a := &ApiServerWatcher{
		l:                log.Logger().Named("apiserver-watcher"),
		hostResolver:     mockedResolver,
		remainingRetries: 3,
	}

	mockedResolver.EXPECT().LookupHost(gomock.Any(), gomock.Any()).Return(nil, errors.New("Error")).AnyTimes()

	a.Refresh(ctx)
	require.NoError(t, a.Refresh(context.Background()), "Expected error when refreshing the cache")
}

func TestInitWithIncorrectURL(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	mockedResolver := mocks.NewMockIHostResolver(ctrl)
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

func TestRefreshFailsOnlyOnFourthAttempt(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	mockedResolver := mocks.NewMockIHostResolver(ctrl)
	mockedFilterManager := filtermanagermocks.NewMockIFilterManager(ctrl)

	a := &ApiServerWatcher{
		l:                log.Logger().Named("apiserver-watcher"),
		hostResolver:     mockedResolver,
		filterManager:    mockedFilterManager,
		remainingRetries: 3, // Set the initial retry count to 3
	}

	// Simulate LookupHost failing for all attempts.
	mockedResolver.EXPECT().LookupHost(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("simulated DNS error")).AnyTimes()

	mockedFilterManager.EXPECT().AddIPs(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mockedFilterManager.EXPECT().DeleteIPs(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	// Call Refresh three times and expect it to succeed (no error)
	for i := 0; i < 3; i++ {
		err := a.Refresh(ctx)
		require.NoError(t, err, "Expected no error on attempt %d", i+1)
	}

	// Call Refresh the fourth time and expect it to fail
	err := a.Refresh(ctx)
	require.Error(t, err, "Expected error on the fourth attempt")
}
