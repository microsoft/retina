package veth

import (
	"context"
)

func (w *Watcher) Name() string {
	return watcherName
}

func (w *Watcher) Start(_ context.Context) error {
	// Not implemented for Windows
	return nil
}

func (w *Watcher) Stop(_ context.Context) error {
	// Not implemented for Windows
	return nil
}
