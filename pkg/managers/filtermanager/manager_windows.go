// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package filtermanager

import (
	"net"
)

// FilterManager is a no-op implementation of the filter manager for Windows.

type FilterManager struct{}

func Init(_ int) (*FilterManager, error) {
	return nil, nil
}

func (f *FilterManager) AddIPs(_ []net.IP, _ Requestor, _ RequestMetadata) error {
	return nil
}

func (f *FilterManager) DeleteIPs(_ []net.IP, _ Requestor, _ RequestMetadata) error {
	return nil
}

func (f *FilterManager) HasIP(_ net.IP) bool {
	return false
}

func (f *FilterManager) Reset() error {
	return nil
}

func (f *FilterManager) Stop() error {
	return nil
}
