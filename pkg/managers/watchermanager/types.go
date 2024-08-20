// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package watchermanager

import (
	"context"

	"github.com/microsoft/retina/pkg/log"
)

//go:generate go run go.uber.org/mock/mockgen@v0.4.0 -source=types.go -destination=mocks/mock_types.go -package=mocks .
type Watcher interface {
	// Start and Stop should only be called by watchermanager.
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Name() string
}

type Manager interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

type WatcherManager struct {
	Watchers []Watcher
	l        *log.ZapLogger
}
