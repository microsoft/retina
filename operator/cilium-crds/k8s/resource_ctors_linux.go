// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Retina and Cilium

package k8s

import (
	operatork8s "github.com/cilium/cilium/operator/k8s"
)

// Re-export Cilium's operator resource constructors.
// These live in a _linux file because cilium/operator/k8s transitively
// imports Linux-only symbols (netns.GetNetNSCookie).
var (
	CiliumEndpointResource      = operatork8s.CiliumEndpointResource
	CiliumEndpointSliceResource = operatork8s.CiliumEndpointSliceResource
	PodResource                 = operatork8s.PodResource
	CiliumEndpointIndexIdentity = operatork8s.CiliumEndpointIndexIdentity
)
