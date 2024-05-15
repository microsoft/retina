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
	switch determineEndian() {
	case binary.BigEndian:
		binary.BigEndian.PutUint32(ip, nn)
	default:
		// default is little endian
		binary.LittleEndian.PutUint32(ip, nn)
	}
	return ip
}

func Ip2int(ip []byte) (res uint32, err error) {
	if len(ip) == 16 {
		return res, errors.New("IPv6 not supported")
	}
	switch determineEndian() {
	case binary.BigEndian:
		res = binary.BigEndian.Uint32(ip)
	default:
		// default is little endian.
		res = binary.LittleEndian.Uint32(ip)
	}
	return res, nil
}

// HostToNetShort converts a 16-bit integer from host to network byte order, aka "htons"
func HostToNetShort(i uint16) uint16 {
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, i)
	return binary.BigEndian.Uint16(b)
}

func determineEndian() binary.ByteOrder {
	var endian binary.ByteOrder
	buf := [2]byte{}
	*(*uint16)(unsafe.Pointer(&buf[0])) = uint16(0xABCD)

	switch buf {
	case [2]byte{0xCD, 0xAB}:
		endian = binary.LittleEndian
	case [2]byte{0xAB, 0xCD}:
		endian = binary.BigEndian
	default:
		fmt.Println("Couldn't determine endianness")
	}
	return endian
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
