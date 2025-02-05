// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package apiserver

import (
	"context"
	"net"
	"time"

	"github.com/microsoft/retina/pkg/log"
	fm "github.com/microsoft/retina/pkg/managers/filtermanager"
)

//go:generate go run go.uber.org/mock/mockgen@v0.4.0 -source=types.go -destination=mocks/mock_types.go -package=mocks .
type IHostResolver interface {
	LookupHost(context context.Context, host string) ([]string, error)
}

const (
	watcherName          = "apiserver-watcher"
	filterManagerRetries = 3
	defaultRefreshRate   = 30 * time.Second
)

type Watcher struct {
	l             *log.ZapLogger
	current       cache
	new           cache
	apiServerURL  string
	hostResolver  IHostResolver
	filtermanager fm.IFilterManager
	refreshRate   time.Duration
}

// NewWatcher creates a new apiserver watcher.
func NewWatcher() *Watcher {
	w := &Watcher{
		l:            log.Logger().Named(watcherName),
		current:      make(cache),
		apiServerURL: getHostURL(),
		hostResolver: net.DefaultResolver,
		refreshRate:  defaultRefreshRate,
	}
	w.filtermanager = w.getFilterManager()
	return w
}

// define cache as a set
type cache map[string]struct{}

func (c cache) deepcopy() cache {
	copy := make(cache)
	for k, v := range c {
		copy[k] = v
	}
	return copy
}
