// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package filtermanager

import (
	"net"
	"sync"
)

var (
	f    *FilterManager
	once sync.Once
)

type FilterManager struct {
}

func Init(retry int) (*FilterManager, error) {
	return nil, nil
}

func (f *FilterManager) AddIPs(ips []net.IP, r Requestor, m RequestMetadata) error {
	return nil
}

func (f *FilterManager) DeleteIPs(ips []net.IP, r Requestor, m RequestMetadata) error {
	return nil
}

func (f *FilterManager) HasIP(ip net.IP) bool {
	return false
}

func (f *FilterManager) Reset() error {
	return nil
}

func (f *FilterManager) Stop() error {
	return nil
}
