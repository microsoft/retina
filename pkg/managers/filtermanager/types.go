// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package filtermanager

import (
	"net"
)

//go:generate mockgen -source=types.go -destination=mock_types.go -package=filtermanager
type ICache interface {
	ips() []net.IP
	reset()
	hasKey(net.IP) bool
	addIP(net.IP, Requestor, RequestMetadata)
	deleteIP(net.IP, Requestor, RequestMetadata) bool
}

type IFilterManager interface {
	// AddIPs adds the given IPs to the filter map (map in kernel space) and the filterCache.
	// If any of the IP cannot be added to the filter map,
	// no entries will be added to the cache.
	// Note: An error returned doesn't guarantee that no IPs
	// were added to the filter map. Caller should retry adding
	// all the IPs again.
	AddIPs([]net.IP, Requestor, RequestMetadata) error
	// DeleteIPs deletes the given IPs from the filter map (map in kernel space) and the filterCache.
	// If any of the IP cannot be deleted from the filter map,
	// no entries will be deleted from the cache.
	// Note: An error returned doesn't guarantee that no IPs
	// were deleted from the filter map. Caller should retry deleting
	// all the IPs again.
	DeleteIPs([]net.IP, Requestor, RequestMetadata) error
	// HasIP returns true if the given IP is in the filterCache.
	HasIP(net.IP) bool
	// Reset the cache and the filter map.
	// Note: An error returned doesn't guarantee that no IPs
	// were deleted from the filter map. Caller should retry Reset
	// again.
	Reset() error
	// Reset the filterManager and close the kernel map.
	// Error indicates issues deleting all entries in the kernel map.
	Stop() error
}

type Requestor string

type RequestMetadata struct {
	// Each trace can have multiple rules.
	RuleID string
}
