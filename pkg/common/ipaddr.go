// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package common

import "net"

func NewIPAddress(ipv4, ipv6 net.IP) *IPAddresses {
	return &IPAddresses{
		IPv4: ipv4,
		IPv6: ipv6,
	}
}

func (i *IPAddresses) AddIPv4(ip net.IP) {
	i.OtherIPv4s = append(i.OtherIPv4s, ip)
}

func (i *IPAddresses) AddIPv6(ip net.IP) {
	i.OtherIPv6s = append(i.OtherIPv6s, ip)
}

func (i *IPAddresses) GetNetIPs() []net.IP {
	ips := []net.IP{}
	if i.IPv4 != nil {
		ips = append(ips, i.IPv4)
	}
	if i.IPv6 != nil {
		ips = append(ips, i.IPv6)
	}
	ips = append(ips, i.OtherIPv4s...)
	ips = append(ips, i.OtherIPv6s...)
	return ips
}

func (i *IPAddresses) GetIPs() []string {
	ips := []string{}
	if i.IPv4 != nil {
		ips = append(ips, i.IPv4.String())
	}
	if i.IPv6 != nil {
		ips = append(ips, i.IPv6.String())
	}
	for _, ip := range i.OtherIPv4s {
		ips = append(ips, ip.String())
	}
	for _, ip := range i.OtherIPv6s {
		ips = append(ips, ip.String())
	}
	return ips
}

func (i *IPAddresses) GetNetIPv4s() []net.IP {
	ips := []net.IP{}
	if i.IPv4 != nil {
		ips = append(ips, i.IPv4)
	}
	ips = append(ips, i.OtherIPv4s...)
	return ips
}

func (i *IPAddresses) GetNetIPv6s() []net.IP {
	ips := []net.IP{}
	if i.IPv6 != nil {
		ips = append(ips, i.IPv6)
	}
	ips = append(ips, i.OtherIPv6s...)
	return ips
}

func (i *IPAddresses) PrimaryIP() string {
	if i.IPv4 != nil {
		return i.IPv4.String()
	}
	if i.IPv6 != nil {
		return i.IPv6.String()
	}
	return ""
}

func (i *IPAddresses) PrimaryNetIP() net.IP {
	if i.IPv4 != nil {
		return i.IPv4
	}
	if i.IPv6 != nil {
		return i.IPv6
	}
	return nil
}

func (i *IPAddresses) DeepCopy() *IPAddresses {
	if i == nil {
		return nil
	}

	newObj := &IPAddresses{
		IPv4: i.IPv4,
		IPv6: i.IPv6,
	}
	if len(i.OtherIPv4s) > 0 {
		newObj.OtherIPv4s = make([]net.IP, len(i.OtherIPv4s))
		copy(newObj.OtherIPv4s, i.OtherIPv4s)
	}

	if len(i.OtherIPv6s) > 0 {
		newObj.OtherIPv6s = make([]net.IP, len(i.OtherIPv6s))
		copy(newObj.OtherIPv6s, i.OtherIPv6s)
	}

	return newObj
}
