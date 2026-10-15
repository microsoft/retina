package k8s

import (
	"context"
	"fmt"
	"log/slog"

	cgmngr "github.com/cilium/cilium/pkg/cgroups/manager"
	cmtypes "github.com/cilium/cilium/pkg/clustermesh/types"
	"github.com/cilium/cilium/pkg/datapath/iptables"
	iptablesfake "github.com/cilium/cilium/pkg/datapath/iptables/fake"
	ipsecfake "github.com/cilium/cilium/pkg/datapath/linux/ipsec/fake"
	ipsectypes "github.com/cilium/cilium/pkg/datapath/linux/ipsec/types"
	"github.com/cilium/cilium/pkg/datapath/tables"
	"github.com/cilium/cilium/pkg/endpointmanager"
	"github.com/cilium/cilium/pkg/endpointstate"
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
	"github.com/cilium/cilium/pkg/k8s/synced"
	k8stables "github.com/cilium/cilium/pkg/k8s/tables"
	k8sTypes "github.com/cilium/cilium/pkg/k8s/types"
	"github.com/cilium/cilium/pkg/k8s/watchers"
	"github.com/cilium/cilium/pkg/labelsfilter"
	"github.com/cilium/cilium/pkg/loadbalancer"
	"github.com/cilium/cilium/pkg/metrics"
	"github.com/cilium/cilium/pkg/node"
	"github.com/cilium/cilium/pkg/policy"
	policycell "github.com/cilium/cilium/pkg/policy/cell"
	"github.com/cilium/cilium/pkg/promise"
	testidentity "github.com/cilium/cilium/pkg/testutils/identity"
	wgfake "github.com/cilium/cilium/pkg/wireguard/fake"
	wgtypes "github.com/cilium/cilium/pkg/wireguard/types"
	"github.com/cilium/hive/cell"
	"github.com/cilium/statedb"
	"k8s.io/client-go/util/workqueue"

	"github.com/microsoft/retina/pkg/common"
	"github.com/microsoft/retina/pkg/pubsub"
)

// Cell provides the Kubernetes watchers needed by Hubble flow enrichment.
// It is broken into sub-cells so that when Cilium changes its DI graph
// upstream, the failing sub-cell immediately identifies which dependency
// boundary broke.
var Cell = cell.Module(
	"k8s-watcher",
	"Kubernetes watchers needed by Hubble flow enrichment",

	infrastructureCell,
	ipcacheCell,
	watcherCell,
	stubsCell,
)

// infrastructureCell provides the pod/namespace StateDB tables, metrics,
// cgroups, and other foundational components that Cilium's internals depend on.
// Breaks when: Cilium adds new table types or changes table registration APIs.
var infrastructureCell = cell.Group(
	k8stables.PodTableCell,
	k8stables.NamespaceTableCell,

	// Node-address table consumed by Cilium's pod watcher.
	cell.Provide(
		tables.NewNodeAddressTable,
		statedb.RWTable[tables.NodeAddress].ToTable,
	),

	// Load-balancer config for the pod watcher and frontend state for the K8s client.
	loadbalancer.ConfigCell,
	cell.Provide(newFrontendsTable),

	// Cluster info
	cell.Provide(func() cmtypes.ClusterInfo { return cmtypes.DefaultClusterInfo }),

	metrics.Cell,
	cgmngr.Cell,
)

// ipcacheCell provides identity allocation and IPCache — the core
// components Hubble uses to enrich flows with pod/namespace metadata.
// Breaks when: Cilium changes identity, IPCache, or LocalNodeSynchronizer APIs.
var ipcacheCell = cell.Group(
	identitycachecell.Cell,
	endpointmanager.Cell,

	cell.Invoke(initLabelFilter),
	cell.Provide(newIPCache),

	node.LocalNodeStoreCell,
	cell.Provide(newNodeSynchronizer),
)

// watcherCell feeds CiliumEndpoint events into IPCache and Service events into
// Retina's Hubble service resolver.
// Breaks when: Cilium changes resource constructor signatures or watcher config.
var watcherCell = cell.Group(
	cell.Provide(newCiliumEndpointResource),
	cell.Provide(newServiceResource),

	cell.Provide(func() watchers.WatcherConfiguration { return &watcherconfig{} }),
	cell.Provide(func() watchers.ResourceGroupFunc { return watcherResourceGroups }),

	synced.Cell,
	watchers.Cell,

	cell.Provide(newAPIServerEventHandler),
	cell.Invoke(subscribeAPIServerEvents),
)

// stubsCell provides no-op implementations for Cilium features that Retina
// doesn't use (policy engine, datapath, wireguard, ipsec, etc.). These
// satisfy DI requirements without any real functionality.
// Breaks when: Cilium adds new required providers or changes interfaces.
var stubsCell = cell.Group(
	// Fake K8s resources for features Retina doesn't watch
	cell.Provide(
		func() resource.Resource[*cilium_api_v2.CiliumNode] {
			return &fakeresource[*cilium_api_v2.CiliumNode]{}
		},
		func() resource.Resource[*cilium_api_v2alpha1.CiliumEndpointSlice] {
			return &fakeresource[*cilium_api_v2alpha1.CiliumEndpointSlice]{}
		},
	),

	// No-op policy/datapath (Retina doesn't use Cilium's policy engine).
	// Uses Cilium's own per-subsystem fake packages where available.
	cell.Provide(
		func() policycell.IdentityUpdater { return &noOpIdentityUpdater{} },
		// Safe only while no consumer calls IPCache.GetNamedPorts: that enables
		// named-port updates, which would call this zero-value Updater and
		// panic. Use policy.NewUpdater if that changes.
		func() *policy.Updater { return &policy.Updater{} },
		func() wgtypes.Config { return wgfake.Config{} },
		func() ipsectypes.Config { return ipsecfake.Config{} },
		func() iptables.Manager { return iptablesfake.NewManager() },
		// endpointmanager.Cell awaits this before starting its periodic controllers.
		func() promise.Promise[endpointstate.Restorer] {
			r, p := promise.New[endpointstate.Restorer]()
			r.Resolve(&fakeRestorer{})
			return p
		},
	),
)

// ============================================================
// Helper functions
// ============================================================

func initLabelFilter(l *slog.Logger) {
	if err := labelsfilter.ParseLabelPrefixCfg(l, nil, nil, ""); err != nil {
		l.Error("Failed to parse label prefix config", "error", err)
	}
}

func newIPCache(l *slog.Logger) *ipcache.IPCache {
	alloc := cache.NewCachingIdentityAllocator(
		l,
		&testidentity.IdentityAllocatorOwnerMock{},
		cache.AllocatorConfig{},
	)
	return ipcache.NewIPCache(&ipcache.Configuration{
		Context:           context.Background(),
		Logger:            l.With("module", "ipcache"),
		IdentityAllocator: alloc,
		IdentityUpdater:   &noOpIdentityUpdater{},
	})
}

func newNodeSynchronizer(l *slog.Logger) node.LocalNodeSynchronizer {
	return &nodeSynchronizer{l: l.With("module", "node-synchronizer")}
}

// CiliumSlimEndpointResource dereferences localNodeStore while indexing each
// endpoint, so nil would panic on the first event.
func newCiliumEndpointResource(
	l *slog.Logger, lc cell.Lifecycle, cs client.Clientset, mp workqueue.MetricsProvider,
	localNodeStore *node.LocalNodeStore,
) (resource.Resource[*k8sTypes.CiliumEndpoint], error) {
	//nolint:wrapcheck // wrapped error here is of dubious value
	return ciliumk8s.CiliumSlimEndpointResource(ciliumk8s.CiliumResourceParams{
		Logger:          l,
		Lifecycle:       lc,
		ClientSet:       cs,
		MetricsProvider: mp,
	}, localNodeStore, mp)
}

func newServiceResource(
	lc cell.Lifecycle, cs client.Clientset, mp workqueue.MetricsProvider,
) (resource.Resource[*slim_corev1.Service], error) {
	//nolint:wrapcheck // wrapped error here is of dubious value
	return ciliumk8s.ServiceResource(
		lc,
		ciliumk8s.ConfigParams{
			Config:      ciliumk8s.Config{K8sServiceProxyName: ""},
			WatchConfig: ciliumk8s.ServiceWatchConfig{EnableHeadlessServiceWatch: false},
		},
		cs,
		mp,
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
// Watcher configuration
// ============================================================

const K8sAPIGroupCiliumEndpointV2 = "cilium/v2::CiliumEndpoint"

var k8sResources = []string{K8sAPIGroupCiliumEndpointV2}

func watcherResourceGroups(*slog.Logger, watchers.WatcherConfiguration) (r, w []string) {
	return k8sResources, w
}

// ============================================================
// No-op identity updater
// ============================================================

type noOpIdentityUpdater struct{}

func (n *noOpIdentityUpdater) UpdateIdentities(_, _ identity.IdentityMap) <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}
