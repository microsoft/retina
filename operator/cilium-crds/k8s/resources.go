// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Retina and Cilium

// NOTE: we've slimmed down the resources required here to:
// - identities
// - endpoints
// - endpointslices (required for identitygc)
// - ciliumnodes (required for endpointgc)

package k8s

import (
	"github.com/cilium/cilium/pkg/k8s"
	cilium_api_v2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	cilium_api_v2alpha1 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2alpha1"
	"github.com/cilium/cilium/pkg/k8s/resource"
	slim_corev1 "github.com/cilium/cilium/pkg/k8s/slim/k8s/api/core/v1"
	"github.com/cilium/hive/cell"
	"k8s.io/client-go/util/workqueue"
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
		// Provide a no-op MetricsProvider for resource.New calls.
		func() workqueue.MetricsProvider { return noopMetricsProvider{} },
		k8s.CiliumIdentityResource,
		CiliumEndpointResource,
		CiliumEndpointSliceResource,
		func() resource.Resource[*cilium_api_v2.CiliumNode] {
			return &fakeresource[*cilium_api_v2.CiliumNode]{}
		},
		PodResource,
		k8s.NamespaceResource,
	),
)

// noopMetricsProvider is a no-op implementation of workqueue.MetricsProvider
// used to satisfy the MetricsProvider parameter required by resource.New in Cilium v1.19.0.
type noopMetricsProvider struct{}

func (noopMetricsProvider) NewDepthMetric(name string) workqueue.GaugeMetric {
	return noopMetric{}
}

func (noopMetricsProvider) NewAddsMetric(name string) workqueue.CounterMetric {
	return noopMetric{}
}

func (noopMetricsProvider) NewLatencyMetric(name string) workqueue.HistogramMetric {
	return noopMetric{}
}

func (noopMetricsProvider) NewWorkDurationMetric(name string) workqueue.HistogramMetric {
	return noopMetric{}
}

func (noopMetricsProvider) NewUnfinishedWorkSecondsMetric(name string) workqueue.SettableGaugeMetric {
	return noopMetric{}
}

func (noopMetricsProvider) NewLongestRunningProcessorSecondsMetric(name string) workqueue.SettableGaugeMetric {
	return noopMetric{}
}

func (noopMetricsProvider) NewRetriesMetric(name string) workqueue.CounterMetric {
	return noopMetric{}
}

type noopMetric struct{}

func (noopMetric) Inc()                             {}
func (noopMetric) Dec()                             {}
func (noopMetric) Set(float64)                      {}
func (noopMetric) Observe(float64)                  {}

// Resources is a convenience struct to group all the operator k8s resources as cell constructor parameters.
type Resources struct {
	cell.In

	Identities           resource.Resource[*cilium_api_v2.CiliumIdentity]
	CiliumEndpoints      resource.Resource[*cilium_api_v2.CiliumEndpoint]
	CiliumEndpointSlices resource.Resource[*cilium_api_v2alpha1.CiliumEndpointSlice]
	CiliumNodes          resource.Resource[*cilium_api_v2.CiliumNode]
	Pods                 resource.Resource[*slim_corev1.Pod]
}
