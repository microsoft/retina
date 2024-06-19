// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package utils

import (
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"syscall"
	"unsafe"

	"github.com/cilium/cilium/api/v1/flow"
	"github.com/pkg/errors"
	"github.com/vishvananda/netlink"
	"golang.org/x/exp/maps"
)

const (
	ipv4ZeroSubnet = "0.0.0.0/0"
	ipv6ZeroSubnet = "::/0"
)

// Both openRawSock and htons are available in
// https://github.com/cilium/ebpf/blob/master/example_sock_elf_test.go.
// MIT license.

func OpenRawSocket(index int) (int, error) {
	sock, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_RAW|syscall.SOCK_NONBLOCK|syscall.SOCK_CLOEXEC, int(htons(syscall.ETH_P_ALL)))
	if err != nil {
		return 0, err
	}
	sll := syscall.SockaddrLinklayer{
		Ifindex:  index,
		Protocol: htons(syscall.ETH_P_ALL),
	}
	if err := syscall.Bind(sock, &sll); err != nil {
		return 0, err
	}
	return sock, nil
}

// htons converts the unsigned short integer hostshort from host byte order to network byte order.
func htons(i uint16) uint16 {
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, i)
	return *(*uint16)(unsafe.Pointer(&b[0]))
}

// https://gist.github.com/ammario/649d4c0da650162efd404af23e25b86b
func Int2ip(nn uint32) net.IP {
	ip := make(net.IP, 4)
	binary.LittleEndian.PutUint32(ip, nn)
	return ip
}

func Ip2int(ip []byte) (res uint32, err error) {
	if len(ip) == 16 {
		return res, errors.New("IPv6 not supported")
	}
	return binary.LittleEndian.Uint32(ip), nil
}

// HostToNetShort converts a 16-bit integer from host to network byte order, aka "htons"
func HostToNetShort(i uint16) uint16 {
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, i)
	return binary.BigEndian.Uint16(b)
}

// GetDefaultOutgoingLinks gets the outgoing interface by executing an equivalent to `ip route show default 0.0.0.0/0`
func GetDefaultOutgoingLinks() ([]netlink.Link, error) {
	routes, err := netlink.RouteList(nil, netlink.FAMILY_ALL)
	if err != nil {
		return nil, fmt.Errorf("failed to get route list: %w", err)
	}

	defaultLinks := make(map[int]netlink.Link)
	for i := range routes {
		routeLinkIndex := routes[i].LinkIndex
		if _, ok := defaultLinks[routeLinkIndex]; ok {
			continue
		}

		if !isDefaultRoute(routes[i]) {
			continue
		}

		link, err := netlink.LinkByIndex(routeLinkIndex)
		if err != nil {
			return nil, fmt.Errorf("failed to get link %d by index: %w", routeLinkIndex, err)
		}

		defaultLinks[routeLinkIndex] = link
	}

	return maps.Values(defaultLinks), nil
}

func isDefaultRoute(route netlink.Route) bool {
	if route.Dst == nil {
		return true
	}

	destination := route.Dst.String()

	if strings.EqualFold(destination, ipv4ZeroSubnet) {
		return true
	}

	if strings.EqualFold(destination, ipv6ZeroSubnet) {
		return true
	}

	return false
}

func GetDropReasonDesc(dr DropReason) flow.DropReason {
	// Set the drop reason.
	// Retina drop reasons are different from the drop reasons available in flow library.
	// We map the ones available in flow library to the ones available in Retina.
	// Rest are set to UNKNOWN. The details are added in the metadata.
	switch dr { //nolint:exhaustive // We are handling all the cases.
	case DropReason_IPTABLE_RULE_DROP:
		return flow.DropReason_POLICY_DENIED
	case DropReason_IPTABLE_NAT_DROP:
		return flow.DropReason_SNAT_NO_MAP_FOUND
	case DropReason_CONNTRACK_ADD_DROP:
		return flow.DropReason_UNKNOWN_CONNECTION_TRACKING_STATE
	default:
		return flow.DropReason_DROP_REASON_UNKNOWN
	}
}
