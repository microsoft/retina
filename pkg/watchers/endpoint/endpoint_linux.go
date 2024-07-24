// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package endpoint

import (
	"github.com/vishvananda/netlink"
)

var showLink = netlink.LinkList

func (e *Watcher) initNewCache() error {
	veths, err := listVeths()
	if err != nil {
		return err
	}

	// Reset new cache.
	e.new = make(cache)
	for _, veth := range veths {
		k := key{
			name:         veth.Attrs().Name,
			hardwareAddr: veth.Attrs().HardwareAddr.String(),
			netNsID:      veth.Attrs().NetNsID,
		}

		e.new[k] = *veth.Attrs()
	}

	return nil
}

// Helper functions.

// Get all the veth interfaces.
// Similar to ip link show type veth
func listVeths() ([]netlink.Link, error) {
	links, err := showLink()
	if err != nil {
		return nil, err
	}

	var veths []netlink.Link
	for _, link := range links {
		// Ref: https://github.com/vishvananda/netlink/blob/ced5aaba43e3f25bb5f04860641d3e3dd04a8544/link.go#L367
		// Unfortunately, there is no type/constant defined for "veth" in the netlink package.
		// Version of netlink tested - https://github.com/vishvananda/netlink/tree/v1.2.1-beta.2
		if link.Type() == "veth" {
			veths = append(veths, link)
		}
	}

	return veths, nil
}
