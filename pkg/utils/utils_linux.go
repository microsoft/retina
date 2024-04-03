// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package utils

import (
	"encoding/binary"
	"fmt"
	"net"
	"syscall"
	"unsafe"

	"github.com/pkg/errors"
	"github.com/vishvananda/netlink"
)

var (
	routeGet    = netlink.RouteGet
	linkByIndex = netlink.LinkByIndex
	showLink    = netlink.LinkList
)

var (
	externalIP   = net.ParseIP("1.1.1.1")
	externalIPv6 = net.ParseIP("2606:4700:4700::1111")
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

// GetOutgoingInterface gets the outgoing interface by first trying
// executing an equivalent to `ip route get <External IP>`
// and fall back to `ip link show type veth`
func GetOutgoingInterface(fallbackInterfaceName string, interfaceType string) (netlink.Link, error) {
	if routes, err := routeGet(externalIP); err == nil {
		for _, route := range routes {
			if link, err := linkByIndex(route.LinkIndex); err == nil && link.Type() == interfaceType {
				return link, nil
			}
		}
	}

	if routes, err := routeGet(externalIPv6); err == nil {
		for _, route := range routes {
			if link, err := linkByIndex(route.LinkIndex); err == nil && link.Type() == interfaceType {
				return link, nil
			}
		}
	}

	links, err := showLink()
	if err != nil {
		return nil, err
	}

	for _, link := range links {
		// Ref for types: https://github.com/vishvananda/netlink/blob/ced5aaba43e3f25bb5f04860641d3e3dd04a8544/link.go#L367
		// Version of netlink tested - https://github.com/vishvananda/netlink/tree/v1.2.1-beta.2
		if link.Type() == interfaceType && link.Attrs().Name == fallbackInterfaceName {
			return link, nil
		}
	}

	return nil, fmt.Errorf("interface %s of type %s not found", fallbackInterfaceName, interfaceType)
}
