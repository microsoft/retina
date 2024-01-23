// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package common

import "net"

func NewRetinaNode(name string, ip net.IP) *RetinaNode {
	return &RetinaNode{
		name: name,
		ip:   ip,
	}
}

func (n *RetinaNode) DeepCopy() interface{} {
	newN := &RetinaNode{
		name: n.name,
	}

	if n.ip != nil {
		newN.ip = make(net.IP, len(n.ip))
		copy(newN.ip, n.ip)
	}

	return newN
}

func (n *RetinaNode) IPString() string {
	return n.ip.String()
}

func (n *RetinaNode) Name() string {
	return n.name
}
