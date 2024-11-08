// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

// Package filter contains the Retina filter plugin. It utilizes eBPF to filter packets based on IP addresses.
package filter

import (
	"errors"
	"net"
	"strings"
	"sync"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/rlimit"
	"github.com/microsoft/retina/pkg/log"
	plugincommon "github.com/microsoft/retina/pkg/plugin/common"
	"github.com/microsoft/retina/pkg/utils"
	"go.uber.org/zap"

	_ "github.com/microsoft/retina/pkg/plugin/filter/_cprog" // nolint
)

//go:generate bpf2go -cflags "-g -O2 -Wall -D__TARGET_ARCH_${GOARCH} -Wall" -target ${GOARCH} -type mapKey filter ./_cprog/retina_filter.c -- -I../lib/_${GOARCH} -I../lib/common/libbpf/_src

var (
	f    *FilterMap
	once sync.Once
)

type FilterMap struct {
	l                    *log.ZapLogger
	obj                  *filterObjects //nolint:typecheck
	kfm                  IEbpfMap
	batchApiNotSupported bool
}

func Init() (*FilterMap, error) {
	once.Do(func() {
		f = &FilterMap{}
	})
	if f.l == nil {
		f.l = log.Logger().Named("filter-map")
	}
	if f.obj != nil {
		return f, nil
	}

	// Allow the current process to lock memory for eBPF resources.
	if err := rlimit.RemoveMemlock(); err != nil {
		f.l.Error("RemoveMemlock failed", zap.Error(err))
		return f, err
	}

	obj := &filterObjects{}                                //nolint:typecheck
	err := loadFilterObjects(obj, &ebpf.CollectionOptions{ //nolint:typecheck
		Maps: ebpf.MapOptions{
			PinPath: plugincommon.MapPath,
		},
	})
	if err != nil {
		f.l.Error("loadFiltermanagerObjects failed", zap.Error(err))
		return f, err
	}
	f.obj = obj
	f.kfm = obj.RetinaFilter
	return f, nil
}

func (f *FilterMap) Add(ips []net.IP) error {
	keys := []filterMapKey{} //nolint:typecheck
	values := make([]uint8, len(ips))
	for idx, ip := range ips {
		key, err := mapKey(ip)
		if err != nil {
			return err
		}
		keys = append(keys, key)
		values[idx] = 1
	}

	var updated int
	var err error
	if !f.batchApiNotSupported {
		updated, err = f.kfm.BatchUpdate(keys, values, &ebpf.BatchOptions{
			Flags: uint64(ebpf.UpdateAny),
		})

		// if batch update not supported by kernel, perform single updates
		if err != nil && strings.Contains(err.Error(), "not supported") {
			f.l.Debug("Batch update not supported by kernel. Performing single updates instead.")
			f.batchApiNotSupported = true
			updated, err = f.performSingleUpdates(keys, values)
			if err != nil {
				return err
			}
		}
	} else {
		updated, err = f.performSingleUpdates(keys, values)
		if err != nil {
			return err
		}
	}

	f.l.Debug("Add", zap.Int("updated", updated))
	return err
}

func (f *FilterMap) Delete(ips []net.IP) error {
	keys := []filterMapKey{} //nolint:typecheck
	for _, ip := range ips {
		key, err := mapKey(ip)
		if err != nil {
			return err
		}
		keys = append(keys, key)
	}
	var deleted int
	var err error
	if !f.batchApiNotSupported {
		deleted, err = f.kfm.BatchDelete(keys, &ebpf.BatchOptions{
			Flags: uint64(ebpf.UpdateAny),
		})

		// if batch delete not supported by kernel, perform single deletes
		if err != nil && strings.Contains(err.Error(), "not supported") {
			f.l.Debug("Batch delete not supported by kernel. Performing single deletes instead.")
			f.batchApiNotSupported = true
			deleted, err = f.performSingleDeletes(keys)
			if err != nil {
				return err
			}
		}
	} else {
		deleted, err = f.performSingleDeletes(keys)
		if err != nil {
			return err
		}
	}

	f.l.Debug("Delete", zap.Int("deleted", deleted))
	return err
}

func (f *FilterMap) performSingleUpdates(keys []filterMapKey, values []uint8) (int, error) { //nolint:typecheck
	var updated int
	for idx, key := range keys {
		err := f.kfm.Put(key, values[idx])
		if err != nil {
			return updated, err
		}
		updated++
	}
	return updated, nil
}

func (f *FilterMap) performSingleDeletes(keys []filterMapKey) (int, error) { //nolint:typecheck
	var deleted int
	for _, key := range keys {
		err := f.kfm.Delete(key)
		if err != nil {
			return deleted, err
		}
		deleted++
	}
	return deleted, nil
}

func (f *FilterMap) Close() {
	if f.obj == nil {
		return
	}
	if f.kfm != nil {
		f.kfm.Close()
	}
	f.obj.Close()
}

// Helper functions.
func mapKey(ip net.IP) (filterMapKey, error) { //nolint:typecheck
	// Convert to 4 byte representation.
	// Sometimes, IP.Net has 16 byte representation for IPV4 addresses.
	ipv4 := ip.To4()
	if ipv4 == nil {
		return filterMapKey{}, errors.New("invalid IP address") //nolint:typecheck
	}
	ipv4Int, err := utils.Ip2int(ipv4)
	if err != nil {
		return filterMapKey{}, err //nolint:typecheck
	}
	return filterMapKey{ //nolint:typecheck
		Prefixlen: uint32(32),
		Data:      ipv4Int,
	}, nil
}
