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
	"go.uber.org/mock/gomock"
)

func TestStart(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	mockedResolver := mocks.NewMockIHostResolver(ctrl)
	mockedFilterManager := filtermanagermocks.NewMockIFilterManager(ctrl)

	w := &Watcher{
		l:             log.Logger().Named(watcherName),
		apiServerURL:  "https://kubernetes.default.svc.cluster.local:443",
		hostResolver:  mockedResolver,
		filtermanager: mockedFilterManager,
		refreshRate:   5 * time.Second,
	}

	// Return 2 random IPs for the host everytime LookupHost is called.
	mockedResolver.EXPECT().LookupHost(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, host string) ([]string, error) {
		return []string{randomIP(), randomIP()}, nil
	}).AnyTimes()

	mockedFilterManager.EXPECT().AddIPs(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mockedFilterManager.EXPECT().DeleteIPs(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	err := w.Start(ctx) // watcher will timeout after 20 seconds
	assert.NoError(t, err, "Expected no error when refreshing the cache")
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

	a := &Watcher{
		l:            log.Logger().Named(watcherName),
		apiServerURL: "https://kubernetes.default.svc.cluster.local:443",
		hostResolver: mockedResolver,
		current:      old,
		new:          new,
		refreshRate:  5 * time.Second,
	}

	created, deleted := a.diffCache()
	assert.Equal(t, 1, len(created), "Expected 1 created host")
	assert.Equal(t, 1, len(deleted), "Expected 1 deleted host")
}

func TestStartError(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	mockedResolver := mocks.NewMockIHostResolver(ctrl)

	w := &Watcher{
		l:            log.Logger().Named(watcherName),
		apiServerURL: "https://kubernetes.default.svc.cluster.local:443",
		hostResolver: mockedResolver,
		refreshRate:  5 * time.Second,
	}

	mockedResolver.EXPECT().LookupHost(gomock.Any(), gomock.Any()).Return(nil, errors.New("Error")).AnyTimes()

	err := w.Start(ctx)
	assert.Error(t, err, "Expected error when refreshing the cache")
}

func TestResolveIPEmpty(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	mockedResolver := mocks.NewMockIHostResolver(ctrl)

	w := &Watcher{
		l:            log.Logger().Named(watcherName),
		apiServerURL: "https://kubernetes.default.svc.cluster.local:443",
		hostResolver: mockedResolver,
		refreshRate:  5 * time.Second,
	}

	mockedResolver.EXPECT().LookupHost(gomock.Any(), gomock.Any()).Return([]string{}, nil).AnyTimes()

	err := w.Start(ctx)
	assert.Error(t, err, "Expected error when refreshing the cache")
}

func randomIP() string {
	return fmt.Sprintf("%d.%d.%d.%d", rand.Intn(256), rand.Intn(256), rand.Intn(256), rand.Intn(256))
}
