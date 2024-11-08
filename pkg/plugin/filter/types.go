// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package filter

import (
	"net"

	"github.com/cilium/ebpf"
)

//go:generate mockgen -source=types.go -destination=mocks/mock_types.go -package=mocks

/*
A thin wrapper around the eBPF map that allows adding and deleting IPv4 addresses.
Adding this separately here because:
- C code uses the lib for generation/compilation
- Plugins import retina_filter.c to use lookup function
*/
type IFilterMap interface {
	Add([]net.IP) error
	Delete([]net.IP) error
	Close()
}

// Interface for eBPF map.
// Added for UTs.
type IEbpfMap interface {
	BatchUpdate(keys, values interface{}, opts *ebpf.BatchOptions) (int, error)
	BatchDelete(keys interface{}, opts *ebpf.BatchOptions) (int, error)
	Put(key, value interface{}) error
	Delete(key interface{}) error
	Close() error
}
