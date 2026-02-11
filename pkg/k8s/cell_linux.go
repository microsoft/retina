package k8s

import (
	"context"
	"fmt"
	"log/slog"

	daemonk8s "github.com/cilium/cilium/daemon/k8s"
	cgmngr "github.com/cilium/cilium/pkg/cgroups/manager"
	"github.com/cilium/cilium/pkg/datapath/iptables/ipset"
	"github.com/cilium/cilium/pkg/datapath/tables"
	datapath "github.com/cilium/cilium/pkg/datapath/types"
	"github.com/cilium/cilium/pkg/endpointmanager"
	"github.com/cilium/cilium/pkg/identity"
	"github.com/cilium/cilium/pkg/identity/cache"
	identitycachecell "github.com/cilium/cilium/pkg/identity/cache/cell"
	"github.com/cilium/cilium/pkg/ipcache"
	ciliumk8s "github.com/cilium/cilium/pkg/k8s"
	cilium_api_v2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	cilium_api_v2alpha1 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2alpha1"
	"github.com/cilium/cilium/pkg/k8s/client"
	"github.com/cilium/cilium/pkg/k8s/resource"
	slim_corev1 "github.com/cilium/cilium/pkg/k8s/slim/k8s/api/core/v1"
	slim_networkingv1 "github.com/cilium/cilium/pkg/k8s/slim/k8s/api/networking/v1"
	"github.com/cilium/cilium/pkg/k8s/synced"
	k8sTypes "github.com/cilium/cilium/pkg/k8s/types"
	"github.com/cilium/cilium/pkg/k8s/watchers"
	"github.com/cilium/cilium/pkg/labelsfilter"
	"github.com/cilium/cilium/pkg/loadbalancer"
	"github.com/cilium/cilium/pkg/metrics"
	"github.com/cilium/cilium/pkg/node"
	"github.com/cilium/cilium/pkg/policy"
	policycell "github.com/cilium/cilium/pkg/policy/cell"
	"github.com/cilium/hive/cell"
	"github.com/cilium/statedb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"

	"github.com/microsoft/retina/pkg/common"
	"github.com/microsoft/retina/pkg/pubsub"
)

var Cell = cell.Module(
	"k8s-watcher",
	"Kubernetes watchers needed by Hubble flow enrichment",

	// ============================================================
	// CORE INFRASTRUCTURE
	// Required by Cilium's internal components
	// ============================================================

	// StateDB tables for pods and namespaces (used by endpoint manager)
	daemonk8s.PodTableCell,
	daemonk8s.NamespaceTableCell,

	// Device table (required by Cilium internals)
	cell.Provide(
		tables.NewDeviceTable,
		statedb.RWTable[*tables.Device].ToTable,
	),

	// Node address table (required by service cache)
	svcCacheCell,

	// Metrics (used by endpoint manager and watchers)
	metrics.Cell,

	// Cgroups manager (required by endpoint manager)
	cgmngr.Cell,

	// ============================================================
	// HUBBLE REQUIREMENTS
	// These components are essential for Hubble flow enrichment
	// ============================================================

	// IPCache: Maps IPs to identities and K8s metadata
	// This is the core component Hubble uses for flow enrichment
	cell.Provide(newIPCache),

	// Identity cache: Manages security identities for endpoints
	// Required for IPCache to resolve identities
	identitycachecell.Cell,

	// Endpoint manager: Tracks local endpoints
	// Used by watchers to populate IPCache with endpoint metadata
	endpointmanager.Cell,

	// Local node store: Tracks the local node's information
	node.LocalNodeStoreCell,
	cell.Provide(newNodeSynchronizer),

	// ============================================================
	// K8S RESOURCE WATCHERS
	// Watch CiliumEndpoint CRDs to populate IPCache
	// ============================================================

	// Initialize label filter (required by endpoint manager)
	cell.Invoke(initLabelFilter),

	// CiliumEndpoint resource (watched to populate IPCache)
	cell.Provide(newCiliumEndpointResource),

	// Service and Endpoints resources (for service resolution)
	cell.Provide(newEndpointsResource),
	cell.Provide(newServiceResource),

	// Watcher configuration and resource groups
	cell.Provide(func() watchers.WatcherConfiguration { return &watcherconfig{} }),
	cell.Provide(func() watchers.ResourceGroupFunc { return watcherResourceGroups }),

	// Synced cell tracks when initial K8s sync is complete
	synced.Cell,

	// The actual K8s watchers (watches CiliumEndpoint)
	watchers.Cell,

	// API server event handler for pubsub integration
	cell.Provide(newAPIServerEventHandler),
	cell.Invoke(subscribeAPIServerEvents),

	// ============================================================
	// STUBS FOR UNUSED CILIUM FEATURES
	// Retina doesn't use these, but Cilium's code requires them
	// ============================================================

	// Loadbalancer config (required by some Cilium cells)
	loadbalancer.ConfigCell,
	cell.Provide(newFrontendsTable),

	// Fake resources for features Retina doesn't use
	cell.Provide(
		func() resource.Resource[*slim_corev1.Namespace] { return &fakeresource[*slim_corev1.Namespace]{} },
		func() daemonk8s.LocalNodeResource { return &fakeresource[*slim_corev1.Node]{} },
		func() daemonk8s.LocalCiliumNodeResource { return &fakeresource[*cilium_api_v2.CiliumNode]{} },
		func() resource.Resource[*slim_networkingv1.NetworkPolicy] { return &fakeresource[*slim_networkingv1.NetworkPolicy]{} },
		func() resource.Resource[*cilium_api_v2.CiliumNetworkPolicy] { return &fakeresource[*cilium_api_v2.CiliumNetworkPolicy]{} },
		func() resource.Resource[*cilium_api_v2.CiliumClusterwideNetworkPolicy] { return &fakeresource[*cilium_api_v2.CiliumClusterwideNetworkPolicy]{} },
		func() resource.Resource[*cilium_api_v2alpha1.CiliumCIDRGroup] { return &fakeresource[*cilium_api_v2alpha1.CiliumCIDRGroup]{} },
		func() resource.Resource[*cilium_api_v2.CiliumCIDRGroup] { return &fakeresource[*cilium_api_v2.CiliumCIDRGroup]{} },
		func() resource.Resource[*cilium_api_v2alpha1.CiliumEndpointSlice] { return &fakeresource[*cilium_api_v2alpha1.CiliumEndpointSlice]{} },
		func() resource.Resource[*cilium_api_v2.CiliumNode] { return &fakeresource[*cilium_api_v2.CiliumNode]{} },
	),

	// No-op implementations for policy/datapath (Retina doesn't use Cilium's policy engine)
	cell.Provide(
		func() policy.PolicyRepository { return &NoOpPolicyRepository{} },
		func() datapath.Orchestrator { return &NoOpOrchestrator{} },
		func() policycell.IdentityUpdater { return &noOpIdentityUpdater{} },
		func() *policy.Updater { return &policy.Updater{} },
		func() datapath.BandwidthManager { return &fakeBandwidthManager{} },
		func() ipset.Manager { return &fakeIpsetMgr{} },
	),
)

// ============================================================
// CELL HELPER FUNCTIONS
// ============================================================

func initLabelFilter(l *slog.Logger) {
	if err := labelsfilter.ParseLabelPrefixCfg(l, nil, nil, ""); err != nil {
		l.Error("Failed to parse label prefix config", "error", err)
	}
}

func newIPCache(l *slog.Logger) *ipcache.IPCache {
	alloc := cache.NewCachingIdentityAllocator(
		l,
		&identityAllocatorOwner{},
		cache.AllocatorConfig{},
	)
	idAlloc := &cachingIdentityAllocator{alloc, nil}
	return ipcache.NewIPCache(&ipcache.Configuration{
		Context:           context.Background(),
		Logger:            l.With("module", "ipcache"),
		IdentityAllocator: idAlloc,
	})
}

func newNodeSynchronizer(l *slog.Logger) node.LocalNodeSynchronizer {
	return &nodeSynchronizer{l: l.With("module", "node-synchronizer")}
}

func newCiliumEndpointResource(lc cell.Lifecycle, cs client.Clientset, mp workqueue.MetricsProvider) (resource.Resource[*k8sTypes.CiliumEndpoint], error) {
	return ciliumk8s.CiliumSlimEndpointResource(ciliumk8s.CiliumResourceParams{
		Lifecycle: lc,
		ClientSet: cs,
	}, nil, mp, func(*metav1.ListOptions) {})
}

func newEndpointsResource(l *slog.Logger, lc cell.Lifecycle, cs client.Clientset, mp workqueue.MetricsProvider) (resource.Resource[*ciliumk8s.Endpoints], error) {
	//nolint:wrapcheck // wrapped error here is of dubious value
	return ciliumk8s.EndpointsResource(l, lc, ciliumk8s.ConfigParams{
		Config:      ciliumk8s.Config{K8sServiceProxyName: ""},
		WatchConfig: ciliumk8s.ServiceWatchConfig{EnableHeadlessServiceWatch: true},
	}, cs, mp)
}

func newServiceResource(lc cell.Lifecycle, cs client.Clientset, mp workqueue.MetricsProvider) (resource.Resource[*slim_corev1.Service], error) {
	//nolint:wrapcheck // wrapped error here is of dubious value
	return ciliumk8s.ServiceResource(
		lc,
		ciliumk8s.ConfigParams{
			Config:      ciliumk8s.Config{K8sServiceProxyName: ""},
			WatchConfig: ciliumk8s.ServiceWatchConfig{EnableHeadlessServiceWatch: false},
		},
		cs,
		mp,
		func(*metav1.ListOptions) {},
	)
}

func newFrontendsTable(cfg loadbalancer.Config, db *statedb.DB) (statedb.Table[*loadbalancer.Frontend], error) {
	tbl, err := loadbalancer.NewFrontendsTable(cfg, db)
	if err != nil {
		return nil, fmt.Errorf("failed to create frontends table: %w", err)
	}
	return tbl, nil
}

func subscribeAPIServerEvents(a *APIServerEventHandler) {
	ps := pubsub.New()
	fn := pubsub.CallBackFunc(a.handleAPIServerEvent)
	uuid := ps.Subscribe(common.PubSubAPIServer, &fn)
	a.l.Info("Subscribed to PubSub APIServer", "uuid", uuid)
}

// ============================================================
// SERVICE CACHE CELL
// ============================================================

var svcCacheCell = cell.Group(
	cell.Provide(func(db *statedb.DB) (statedb.RWTable[tables.NodeAddress], error) {
		return statedb.NewTable(db, tables.NodeAddressTableName, tables.NodeAddressIndex)
	}),
	cell.Provide(statedb.RWTable[tables.NodeAddress].ToTable),
)

// ============================================================
// WATCHER CONFIGURATION
// ============================================================

const K8sAPIGroupCiliumEndpointV2 = "cilium/v2::CiliumEndpoint"

// k8sResources defines which resources the watcher monitors.
// Only CiliumEndpoint is needed for Hubble flow enrichment.
var k8sResources = []string{K8sAPIGroupCiliumEndpointV2}

// watcherResourceGroups returns the resource groups to watch.
// Retina only needs CiliumEndpoint for IPCache population.
func watcherResourceGroups(*slog.Logger, watchers.WatcherConfiguration) (r, w []string) {
	return k8sResources, w
}

// ============================================================
// NO-OP IDENTITY UPDATER
// ============================================================

// noOpIdentityUpdater is a no-op implementation of policycell.IdentityUpdater.
// Retina doesn't use Cilium's policy features.
type noOpIdentityUpdater struct{}

func (n *noOpIdentityUpdater) UpdateIdentities(_, _ identity.IdentityMap) <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}
