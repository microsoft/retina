// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Hubble

// Copyright Authors of Cilium

package windowsebpf

import (
	"net"
	"net/netip"
	"time"

	flowpb "github.com/cilium/cilium/api/v1/flow"
	"github.com/cilium/cilium/api/v1/models"
	"github.com/cilium/cilium/pkg/cgroups/manager"
	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	"github.com/cilium/cilium/pkg/identity"
	"github.com/cilium/cilium/pkg/ipcache"
	slim_corev1 "github.com/cilium/cilium/pkg/k8s/slim/k8s/api/core/v1"
	"github.com/cilium/cilium/pkg/labels"
	"github.com/cilium/cilium/pkg/policy"
)

// FakeFQDNCache is used for unit tests that needs FQDNCache and/or DNSGetter.
type FakeFQDNCache struct {
	OnInitializeFrom func(entries []*models.DNSLookup)
	OnAddDNSLookup   func(epID uint32, lookupTime time.Time, domainName string, ips []net.IP, ttl uint32)
	OnGetNamesOf     func(epID uint32, ip netip.Addr) []string
}

// InitializeFrom implements FQDNCache.InitializeFrom.
func (f *FakeFQDNCache) InitializeFrom(entries []*models.DNSLookup) {
	if f.OnInitializeFrom != nil {
		f.OnInitializeFrom(entries)
		return
	}
	panic("InitializeFrom([]*models.DNSLookup) should not have been called since it was not defined")
}

// AddDNSLookup implements FQDNCache.AddDNSLookup.
func (f *FakeFQDNCache) AddDNSLookup(epID uint32, lookupTime time.Time, domainName string, ips []net.IP, ttl uint32) {
	if f.OnAddDNSLookup != nil {
		f.OnAddDNSLookup(epID, lookupTime, domainName, ips, ttl)
		return
	}
	panic("AddDNSLookup(uint32, time.Time, string, []net.IP, uint32) should not have been called since it was not defined")
}

// GetNamesOf implements FQDNCache.GetNameOf.
func (f *FakeFQDNCache) GetNamesOf(epID uint32, ip netip.Addr) []string {
	if f.OnGetNamesOf != nil {
		return f.OnGetNamesOf(epID, ip)
	}
	panic("GetNamesOf(uint32, netip.Addr) should not have been called since it was not defined")
}

// NoopDNSGetter always returns an empty response.
var NoopDNSGetter = FakeFQDNCache{
	OnGetNamesOf: func(_ uint32, _ netip.Addr) (fqdns []string) {
		return nil
	},
}

// FakeEndpointGetter is used for unit tests that needs EndpointGetter.
type FakeEndpointGetter struct {
	OnGetEndpointInfo     func(ip netip.Addr) (endpoint v1.EndpointInfo, ok bool)
	OnGetEndpointInfoByID func(id uint16) (endpoint v1.EndpointInfo, ok bool)
}

// GetEndpointInfo implements EndpointGetter.GetEndpointInfo.
func (f *FakeEndpointGetter) GetEndpointInfo(ip netip.Addr) (endpoint v1.EndpointInfo, ok bool) {
	if f.OnGetEndpointInfo != nil {
		return f.OnGetEndpointInfo(ip)
	}
	panic("OnGetEndpointInfo not set")
}

// GetEndpointInfoByID implements EndpointGetter.GetEndpointInfoByID.
func (f *FakeEndpointGetter) GetEndpointInfoByID(id uint16) (endpoint v1.EndpointInfo, ok bool) {
	if f.OnGetEndpointInfoByID != nil {
		return f.OnGetEndpointInfoByID(id)
	}
	panic("GetEndpointInfoByID not set")
}

// NoopEndpointGetter always returns an empty response.
var NoopEndpointGetter = FakeEndpointGetter{
	OnGetEndpointInfo: func(_ netip.Addr) (endpoint v1.EndpointInfo, ok bool) {
		return nil, false
	},
	OnGetEndpointInfoByID: func(_ uint16) (endpoint v1.EndpointInfo, ok bool) {
		return nil, false
	},
}

type FakeLinkGetter struct{}

func (e *FakeLinkGetter) Name(_ uint32) string {
	return "lo"
}

func (e *FakeLinkGetter) GetIfNameCached(ifindex int) (string, bool) {
	return e.Name(uint32(ifindex)), true //nolint:gosec // this is a noop
}

var NoopLinkGetter = FakeLinkGetter{}

// FakeIPGetter is used for unit tests that needs IPGetter.
type FakeIPGetter struct {
	OnGetK8sMetadata  func(ip netip.Addr) *ipcache.K8sMetadata
	OnLookupSecIDByIP func(ip netip.Addr) (ipcache.Identity, bool)
}

// GetK8sMetadata implements FakeIPGetter.GetK8sMetadata.
func (f *FakeIPGetter) GetK8sMetadata(ip netip.Addr) *ipcache.K8sMetadata {
	if f.OnGetK8sMetadata != nil {
		return f.OnGetK8sMetadata(ip)
	}
	panic("OnGetK8sMetadata not set")
}

// LookupSecIDByIP implements FakeIPGetter.LookupSecIDByIP.
func (f *FakeIPGetter) LookupSecIDByIP(ip netip.Addr) (ipcache.Identity, bool) {
	if f.OnLookupSecIDByIP != nil {
		return f.OnLookupSecIDByIP(ip)
	}
	panic("OnLookupByIP not set")
}

// NoopIPGetter always returns an empty response.
var NoopIPGetter = FakeIPGetter{
	OnGetK8sMetadata: func(_ netip.Addr) *ipcache.K8sMetadata {
		return nil
	},
	OnLookupSecIDByIP: func(_ netip.Addr) (ipcache.Identity, bool) {
		return ipcache.Identity{}, false
	},
}

// FakeServiceGetter is used for unit tests that need ServiceGetter.
type FakeServiceGetter struct {
	OnGetServiceByAddr func(ip netip.Addr, port uint16) *flowpb.Service
}

// GetServiceByAddr implements FakeServiceGetter.GetServiceByAddr.
func (f *FakeServiceGetter) GetServiceByAddr(ip netip.Addr, port uint16) *flowpb.Service {
	if f.OnGetServiceByAddr != nil {
		return f.OnGetServiceByAddr(ip, port)
	}
	panic("OnGetServiceByAddr not set")
}

// NoopServiceGetter always returns an empty response.
var NoopServiceGetter = FakeServiceGetter{
	OnGetServiceByAddr: func(_ netip.Addr, _ uint16) *flowpb.Service {
		return nil
	},
}

// FakeIdentityGetter is used for unit tests that need IdentityGetter.
type FakeIdentityGetter struct {
	OnGetIdentity func(securityIdentity uint32) (*identity.Identity, error)
}

// GetIdentity implements IdentityGetter.GetIPIdentity.
func (f *FakeIdentityGetter) GetIdentity(securityIdentity uint32) (*identity.Identity, error) {
	if f.OnGetIdentity != nil {
		return f.OnGetIdentity(securityIdentity)
	}
	panic("OnGetIdentity not set")
}

// NoopIdentityGetter always returns an empty response.
var NoopIdentityGetter = FakeIdentityGetter{
	OnGetIdentity: func(_ uint32) (*identity.Identity, error) {
		return &identity.Identity{}, nil
	},
}

// FakeEndpointInfo implements v1.EndpointInfo for unit tests. All interface
// methods return values exposed in the fields.
type FakeEndpointInfo struct {
	ContainerIDs []string
	ID           uint64
	Identity     identity.NumericIdentity
	IPv4         net.IP
	IPv6         net.IP
	PodName      string
	PodNamespace string
	Labels       []string
	Pod          *slim_corev1.Pod

	PolicyMap      map[policy.Key]labels.LabelArrayList
	PolicyRevision uint64
}

// GetID returns the ID of the endpoint.
func (e *FakeEndpointInfo) GetID() uint64 {
	return e.ID
}

// GetIdentity returns the numerical security identity of the endpoint.
func (e *FakeEndpointInfo) GetIdentity() identity.NumericIdentity {
	return e.Identity
}

// GetK8sPodName returns the pod name of the endpoint.
func (e *FakeEndpointInfo) GetK8sPodName() string {
	return e.PodName
}

// GetK8sNamespace returns the pod namespace of the endpoint.
func (e *FakeEndpointInfo) GetK8sNamespace() string {
	return e.PodNamespace
}

// GetLabels returns the labels of the endpoint.
func (e *FakeEndpointInfo) GetLabels() []string {
	return e.Labels
}

// GetPod return the pod object of the endpoint.
func (e *FakeEndpointInfo) GetPod() *slim_corev1.Pod {
	return e.Pod
}

func (e *FakeEndpointInfo) GetRealizedPolicyRuleLabelsForKey(key policy.Key) (
	derivedFrom labels.LabelArrayList,
	revision uint64,
	ok bool,
) {
	derivedFrom, ok = e.PolicyMap[key]
	return derivedFrom, e.PolicyRevision, ok
}

// FakePodMetadataGetter is used for unit tests that need a PodMetadataGetter.
type FakePodMetadataGetter struct{}

// GetPodMetadataForContainer implements getters.PodMetadataGetter.
func (f *FakePodMetadataGetter) GetPodMetadataForContainer(_ uint64) *manager.PodMetadata {
	panic("unimplemented")
}

// NoopPodMetadataGetter always returns an empty response.
var NoopPodMetadataGetter = FakePodMetadataGetter{}
