// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

/* Template */

package endpoint

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
