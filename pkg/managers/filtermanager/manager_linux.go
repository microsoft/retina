// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package filtermanager

import (
	"net"
	"sync"

	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/plugin/filter"
	"github.com/microsoft/retina/pkg/utils"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

var (
	f    *FilterManager
	once sync.Once
)

type FilterManager struct {
	mu sync.Mutex
	l  *log.ZapLogger
	c  ICache
	fm filter.IFilterMap
	// retry is the number of times to retry adding/deleting IPs to/from the filter map.
	retry int
}

// Init returns a new instance of FilterManager.
// It is a singleton.
// retry is the number of times to retry adding/deleting IPs to/from the filter map.
// Retry is exponential backoff with a base of 2. For example, retry=3 means:
// 1st try: 1 second
// 2nd try: 2 seconds
// 3rd try: 4 seconds
// Total time: 7 seconds
// The manager locks the cache during retry.
// Suggest to keep retry to a small number (not more than 3).
func Init(retry int) (*FilterManager, error) {
	var err error
	if retry < 1 {
		return nil, errors.New("retry should be greater than 0")
	}
	once.Do(func() {
		f = &FilterManager{
			retry: retry,
		}
	})

	if f.l == nil {
		f.l = log.Logger().Named("filter-manager")
	}
	if f.c == nil {
		f.c = newCache()
	}
	f.fm, err = filter.Init()
	return f, errors.Wrapf(err, "failed to initialize filter map")
}

func (f *FilterManager) AddIPs(ips []net.IP, r Requestor, m RequestMetadata) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	fn := func() error {
		ipsToAdd := []net.IP{}
		for _, ip := range ips {
			if f.c.hasKey(ip) {
				// IP already in filter map, add the request to the cache.
				f.c.addIP(ip, r, m)
			} else {
				ipsToAdd = append(ipsToAdd, ip)
			}
		}
		if len(ipsToAdd) > 0 {
			// Bulk add the IPs to the filter map.
			// This is more efficient than adding one IP at a time.
			err := f.fm.Add(ipsToAdd)
			if err != nil {
				f.l.Error("AddIPs failed", zap.Error(err))
				return err
			}
			// Add the requests to the cache.
			for _, ip := range ipsToAdd {
				f.c.addIP(ip, r, m)
			}
		}
		return nil
	}
	// Exponential backoff retry.
	err := utils.Retry(fn, f.retry)
	return err
}

func (f *FilterManager) DeleteIPs(ips []net.IP, r Requestor, m RequestMetadata) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	fn := func() error {
		ipsToDelete := []net.IP{}
		for _, ip := range ips {
			if f.c.deleteIP(ip, r, m) {
				// No more requests for this IP. Delete it from the filter map.
				ipsToDelete = append(ipsToDelete, ip)
			}
		}
		if len(ipsToDelete) > 0 {
			err := f.fm.Delete(ipsToDelete)
			if err != nil {
				f.l.Error("DeleteIPs failed", zap.Error(err))
				// IPs were not deleted from the filter map. Add them back to the cache.
				for _, ip := range ipsToDelete {
					f.c.addIP(ip, r, m)
				}
				return err
			}
		}
		return nil
	}
	// Exponential backoff retry.
	err := utils.Retry(fn, f.retry)
	return err
}

func (f *FilterManager) HasIP(ip net.IP) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.c.hasKey(ip)
}

func (f *FilterManager) Reset() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	ipsToDelete := f.c.ips()
	if len(ipsToDelete) > 0 {
		err := f.fm.Delete(ipsToDelete)
		if err != nil {
			f.l.Error("Reset failed", zap.Error(err))
			return err
		}
		f.c.reset()
	}
	return nil
}

func (f *FilterManager) Stop() error {
	// Delete all the IPs from the filter map.
	err := f.Reset()
	if err != nil {
		f.l.Error("Reset failed", zap.Error(err))
		return err
	}
	// Close the filter map.
	f.fm.Close()
	// Clear the cache.
	f.c = nil
	return nil
}
