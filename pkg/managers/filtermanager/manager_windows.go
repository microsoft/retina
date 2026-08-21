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
	mu sync.Mutex
	c  ICache
}

func Init(_ int, _ uint32) (*FilterManager, error) {
	once.Do(func() {
		f = &FilterManager{
			c: getCache(),
		}
	})
	return f, nil
}

func (f *FilterManager) AddIPs(ips []net.IP, r Requestor, m RequestMetadata) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, ip := range ips {
		f.c.addIP(ip, r, m)
	}
	return nil
}

func (f *FilterManager) DeleteIPs(ips []net.IP, r Requestor, m RequestMetadata) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, ip := range ips {
		f.c.deleteIP(ip, r, m)
	}
	return nil
}

func (f *FilterManager) HasIP(ip net.IP) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.c.hasKey(ip)
}

func (f *FilterManager) Reset() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.c.reset()
	return nil
}

func (f *FilterManager) Stop() error {
	return f.Reset()
}
