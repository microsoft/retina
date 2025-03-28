package k8s

import (
	"context"
	"io"
	"net"
	"net/netip"
	"sync"

	"github.com/cilium/cilium/api/v1/models"
	"github.com/cilium/cilium/pkg/container/set"
	"github.com/cilium/cilium/pkg/crypto/certificatemanager"
	"github.com/cilium/cilium/pkg/datapath/iptables/ipset"
	"github.com/cilium/cilium/pkg/datapath/loader/metrics"
	datapathtypes "github.com/cilium/cilium/pkg/datapath/types"
	"github.com/cilium/cilium/pkg/identity"
	"github.com/cilium/cilium/pkg/identity/cache"
	"github.com/cilium/cilium/pkg/ipcache"
	ipcachetypes "github.com/cilium/cilium/pkg/ipcache/types"
	"github.com/cilium/cilium/pkg/k8s/resource"
	"github.com/cilium/cilium/pkg/labels"
	"github.com/cilium/cilium/pkg/policy"
	"github.com/cilium/cilium/pkg/policy/api"
	cilium "github.com/cilium/proxy/go/cilium/api"
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

func (w *watcherconfig) KVstoreEnabled() bool {
	return false
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

func (w *internalconfigs) KVstoreEnabledWithoutPodNetworkSupport() bool {
	return false
}

type identityAllocatorOwner struct{}

func (i *identityAllocatorOwner) UpdateIdentities(identity.IdentityMap, identity.IdentityMap) {}

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

func (p *policyhandler) UpdateIdentities(identity.IdentityMap, identity.IdentityMap, *sync.WaitGroup) bool {
	return false
}

type datapathhandler struct{}

func (d *datapathhandler) UpdatePolicyMaps(context.Context, *sync.WaitGroup) *sync.WaitGroup {
	return &sync.WaitGroup{}
}

type fakeBandwidthManager struct{}

func (f *fakeBandwidthManager) BBREnabled() bool {
	return false
}

func (f *fakeBandwidthManager) Enabled() bool {
	return false
}

func (f *fakeBandwidthManager) UpdateBandwidthLimit(uint16, uint64, uint32) {}

func (f *fakeBandwidthManager) DeleteBandwidthLimit(uint16) {}

func (f *fakeBandwidthManager) UpdateIngressBandwidthLimit(endpointID uint16, bytesPerSecond uint64) {
}

func (f *fakeBandwidthManager) DeleteIngressBandwidthLimit(endpointID uint16) {}

type fakeIpsetMgr struct{}

func (f *fakeIpsetMgr) NewInitializer() ipset.Initializer {
	return nil
}

func (f *fakeIpsetMgr) AddToIPSet(string, ipset.Family, ...netip.Addr) {}

func (f *fakeIpsetMgr) RemoveFromIPSet(string, ...netip.Addr) {}

// NoOpPolicyRepository is a no-op implementation of the PolicyRepository interface.
type NoOpPolicyRepository struct{}

func (n *NoOpPolicyRepository) BumpRevision() uint64 {
	return 0
}

func (n *NoOpPolicyRepository) GetAuthTypes(identity.NumericIdentity, identity.NumericIdentity) policy.AuthTypes {
	return policy.AuthTypes{}
}

func (n *NoOpPolicyRepository) GetEnvoyHTTPRules(*api.L7Rules, string) (*cilium.HttpNetworkPolicyRules, bool) {
	return nil, false
}

func (n *NoOpPolicyRepository) GetSelectorPolicy(*identity.Identity, uint64, policy.GetPolicyStatistics, uint64) (policy.SelectorPolicy, uint64, error) {
	return nil, 0, nil
}

func (n *NoOpPolicyRepository) GetRevision() uint64 {
	return 0
}

func (n *NoOpPolicyRepository) GetRulesList() *models.Policy {
	return &models.Policy{}
}

func (n *NoOpPolicyRepository) GetSelectorCache() *policy.SelectorCache {
	return nil
}

func (n *NoOpPolicyRepository) Iterate(f func(*api.Rule)) {}

func (n *NoOpPolicyRepository) ReplaceByResource(api.Rules, ipcachetypes.ResourceID) (affectedIDs *set.Set[identity.NumericIdentity], rev uint64, oldRevCnt int) {
	return nil, 0, 0
}

func (n *NoOpPolicyRepository) ReplaceByLabels(api.Rules, []labels.LabelArray) (affectedIDs *set.Set[identity.NumericIdentity], rev uint64, oldRevCnt int) {
	return nil, 0, 0
}

func (n *NoOpPolicyRepository) Search(labels.LabelArray) (api.Rules, uint64) {
	return nil, 0
}

func (n *NoOpPolicyRepository) SetEnvoyRulesFunc(f func(certificatemanager.SecretManager, *api.L7Rules, string, string) (*cilium.HttpNetworkPolicyRules, bool)) {
}

type NoOpOrchestrator struct{}

func (n *NoOpOrchestrator) Reinitialize(context.Context) error {
	return nil
}

func (n *NoOpOrchestrator) ReloadDatapath(context.Context, datapathtypes.Endpoint, *metrics.SpanStat) (string, error) {
	return "", nil
}

func (n *NoOpOrchestrator) ReinitializeXDP(context.Context, []string) error {
	return nil
}

func (n *NoOpOrchestrator) EndpointHash(datapathtypes.EndpointConfiguration) (string, error) {
	return "", nil
}

func (n *NoOpOrchestrator) WriteEndpointConfig(io.Writer, datapathtypes.EndpointConfiguration) error {
	return nil
}

func (n *NoOpOrchestrator) Unload(datapathtypes.Endpoint) {}
