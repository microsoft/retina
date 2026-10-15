// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package log

import "sync"

// resetGlobalForTest clears the zap global and the self-init once so each test
// can call SetupZapLogger with its own options. Tests in this package do not
// run in parallel, so no locking is required beyond the atomic store.
func resetGlobalForTest() {
	global.Store(nil)
	initOnce = sync.Once{}
}
