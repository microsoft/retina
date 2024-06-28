package k8s

import (
	"context"
	"net"
	"net/netip"
	"sync"
	"time"

	datapathtypes "github.com/cilium/cilium/pkg/datapath/types"
	"github.com/cilium/cilium/pkg/endpoint"
	"github.com/cilium/cilium/pkg/identity"
	"github.com/cilium/cilium/pkg/identity/cache"
	"github.com/cilium/cilium/pkg/ipcache"
	"github.com/cilium/cilium/pkg/k8s/resource"
	slim_corev1 "github.com/cilium/cilium/pkg/k8s/slim/k8s/api/core/v1"
	nodetypes "github.com/cilium/cilium/pkg/node/types"
	k8sRuntime "k8s.io/apimachinery/pkg/runtime"
)

type fakeresource[T k8sRuntime.Object] struct{}

func (f *fakeresource[T]) Events(_ context.Context, _ ...resource.EventsOpt) <-chan resource.Event[T] {
	return make(<-chan resource.Event[T])
}

func (f *fakeresource[T]) Store(_ context.Context) (resource.Store[T], error) {
	return nil, nil
}

func (f *fakeresource[T]) Observe(context.Context, func(resource.Event[T]), func(error)) {
}

type watcherconfig struct {
	internalconfigs
}

type internalconfigs struct{}

func (w *internalconfigs) K8sNetworkPolicyEnabled() bool {
	return false
}

func (w *internalconfigs) K8sIngressControllerEnabled() bool {
	return false
}

func (w *internalconfigs) K8sGatewayAPIEnabled() bool {
	return false
}

type epmgr struct{}

func (e *epmgr) LookupCEPName(string) *endpoint.Endpoint {
	return nil
}

func (e *epmgr) GetEndpoints() []*endpoint.Endpoint {
	return nil
}

func (e *epmgr) GetHostEndpoint() *endpoint.Endpoint {
	return nil
}

func (e *epmgr) GetEndpointsByPodName(string) []*endpoint.Endpoint {
	return nil
}

func (e *epmgr) WaitForEndpointsAtPolicyRev(context.Context, uint64) error {
	return nil
}

func (e *epmgr) UpdatePolicyMaps(context.Context, *sync.WaitGroup) *sync.WaitGroup {
	return nil
}

type nodediscovermgr struct{}

func (n *nodediscovermgr) WaitForLocalNodeInit() {}

func (n *nodediscovermgr) NodeDeleted(nodetypes.Node) {}

func (n *nodediscovermgr) NodeUpdated(nodetypes.Node) {}

func (n *nodediscovermgr) ClusterSizeDependantInterval(time.Duration) time.Duration {
	return time.Duration(0)
}

type cgrpmgr struct{}

func (c *cgrpmgr) OnAddPod(*slim_corev1.Pod) {}

func (c *cgrpmgr) OnUpdatePod(*slim_corev1.Pod, *slim_corev1.Pod) {}

func (c *cgrpmgr) OnDeletePod(*slim_corev1.Pod) {}

type nodeaddressing struct{}

func (n *nodeaddressing) IPv6() datapathtypes.NodeAddressingFamily {
	return nil
}

func (n *nodeaddressing) IPv4() datapathtypes.NodeAddressingFamily {
	return nil
}

type identityAllocatorOwner struct{}

func (i *identityAllocatorOwner) UpdateIdentities(cache.IdentityCache, cache.IdentityCache) {}

func (i *identityAllocatorOwner) GetNodeSuffix() string {
	return ""
}

type cachingIdentityAllocator struct {
	*cache.CachingIdentityAllocator
	ipcache *ipcache.IPCache
}

func (c cachingIdentityAllocator) AllocateCIDRsForIPs([]net.IP, map[netip.Prefix]*identity.Identity) ([]*identity.Identity, error) {
	return nil, nil
}

func (c cachingIdentityAllocator) ReleaseCIDRIdentitiesByID(context.Context, []identity.NumericIdentity) {
}

type policyhandler struct{}

func (p *policyhandler) UpdateIdentities(cache.IdentityCache, cache.IdentityCache, *sync.WaitGroup) {}

type datapathhandler struct{}

func (d *datapathhandler) UpdatePolicyMaps(context.Context, *sync.WaitGroup) *sync.WaitGroup {
	return &sync.WaitGroup{}
}
