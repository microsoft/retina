// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Retina and Cilium

// NOTE: we've slimmed down the resources required here to:
// - identities
// - endpoints
// - endpointslices (required for identitygc)
// - ciliumnodes (required for endpointgc)

package k8s

import (
	"github.com/cilium/cilium/pkg/hive/cell"
	"github.com/cilium/cilium/pkg/k8s"
	cilium_api_v2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	cilium_api_v2alpha1 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2alpha1"
	"github.com/cilium/cilium/pkg/k8s/resource"
	slim_corev1 "github.com/cilium/cilium/pkg/k8s/slim/k8s/api/core/v1"
)

const (
	CiliumEndpointIndexIdentity = "identity"
)

// ResourcesCell provides a set of handles to Kubernetes resources used throughout the
// operator. Each of the resources share a client-go informer and backing store so we only
// have one watch API call for each resource kind and that we maintain only one copy of each object.
//
// See pkg/k8s/resource/resource.go for documentation on the Resource[T] type.
var ResourcesCell = cell.Module(
	"k8s-resources",
	"Operator Kubernetes resources",

	cell.Config(k8s.DefaultConfig),
	cell.Provide(
		k8s.CiliumIdentityResource,
		CiliumEndpointResource,
		CiliumEndpointSliceResource,
		func() resource.Resource[*cilium_api_v2.CiliumNode] {
			return &fakeresource[*cilium_api_v2.CiliumNode]{}
		},
		k8s.PodResource,
		k8s.NamespaceResource,
	),
)

// Resources is a convenience struct to group all the operator k8s resources as cell constructor parameters.
type Resources struct {
	cell.In

	Identities           resource.Resource[*cilium_api_v2.CiliumIdentity]
	CiliumEndpoints      resource.Resource[*cilium_api_v2.CiliumEndpoint]
	CiliumEndpointSlices resource.Resource[*cilium_api_v2alpha1.CiliumEndpointSlice]
	CiliumNodes          resource.Resource[*cilium_api_v2.CiliumNode]
	Pods                 resource.Resource[*slim_corev1.Pod]
}
