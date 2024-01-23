// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package filtermanager

import (
	"net"
	"sync"
)

var fc *filterCache

// requests maps a requestor to a list of request metadata.
// Nested maps
type requests map[Requestor]map[RequestMetadata]bool

type filterCache struct {
	mu sync.Mutex
	// Map of IPs --> Requestor --> RequestMetadata.
	// Each IP can be requested by multiple requestors,
	// for various tasks. For ex:
	// trace1 can request IP1 for rule1 and rule2, while
	// trace2 can request IP1 for rule3.
	data map[string]requests
}

func newCache() *filterCache {
	if fc == nil {
		fc = &filterCache{data: make(map[string]requests)}
	}
	return fc
}

func (f *filterCache) ips() []net.IP {
	f.mu.Lock()
	defer f.mu.Unlock()

	ips := []net.IP{}
	for ip := range f.data {
		ips = append(ips, net.ParseIP(ip))
	}
	return ips
}

func (f *filterCache) reset() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.data = make(map[string]requests)
}

func (f *filterCache) hasKey(ip net.IP) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.data[ip.String()]; ok {
		return true
	}
	return false
}

func (f *filterCache) addIP(ip net.IP, r Requestor, m RequestMetadata) {
	f.mu.Lock()
	defer f.mu.Unlock()

	key := ip.String()
	if _, ok := f.data[key]; !ok {
		f.data[key] = make(requests)
	}
	if _, ok := f.data[key][r]; !ok {
		f.data[key][r] = make(map[RequestMetadata]bool)
	}
	f.data[key][r][m] = true
}

// Return true if the IP was deleted from the cache.
// If more that one requestor has requested the IP, only the requestor
// and the metadata is deleted, and deleteIP returns false.
func (f *filterCache) deleteIP(ip net.IP, r Requestor, m RequestMetadata) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	ipDeleted := false
	key := ip.String()
	if _, ok := f.data[key]; ok {
		if _, ok := f.data[key][r]; ok {
			delete(f.data[key][r], m)
			// If requestor has no more requests for this IP, delete the requestor.
			if len(f.data[key][r]) == 0 {
				delete(f.data[key], r)
			}
			// If IP has no more requests, delete the IP.
			if len(f.data[key]) == 0 {
				delete(f.data, key)
				ipDeleted = true
			}
		}
	}
	return ipDeleted
}
