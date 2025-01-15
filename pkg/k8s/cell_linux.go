package k8s

import (
	"context"

	daemonk8s "github.com/cilium/cilium/daemon/k8s"
	"github.com/cilium/cilium/pkg/bgp/speaker"
	cgmngr "github.com/cilium/cilium/pkg/cgroups/manager"
	"github.com/cilium/cilium/pkg/datapath/iptables/ipset"
	"github.com/cilium/cilium/pkg/datapath/tables"
	"github.com/cilium/cilium/pkg/datapath/types"
	"github.com/cilium/cilium/pkg/endpointmanager"
	"github.com/cilium/cilium/pkg/identity/cache"
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
	"github.com/cilium/cilium/pkg/metrics"
	"github.com/cilium/cilium/pkg/node"
	"github.com/cilium/cilium/pkg/node/manager"
	"github.com/cilium/cilium/pkg/policy"
	"github.com/cilium/cilium/pkg/redirectpolicy"
	"github.com/cilium/cilium/pkg/service"
	"github.com/cilium/hive/cell"
	"github.com/cilium/statedb"
	"github.com/microsoft/retina/pkg/common"
	"github.com/microsoft/retina/pkg/pubsub"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var Cell = cell.Module(
	"k8s-watcher",
	"Kubernetes watchers needed by the agent",

	cell.Provide(
		func() (statedb.Table[tables.NodeAddress], error) {
			return statedb.NewTable(tables.NodeAddressTableName, tables.NodeAddressIndex)
		},
	),
	cell.Invoke(func(db *statedb.DB, t statedb.Table[tables.NodeAddress]) {
		err := db.RegisterTable(t)
		if err != nil {
			logrus.WithError(err).Fatal("Failed to register table")
		}
	}),
	cell.Provide(ciliumk8s.NewServiceCache),

	cell.Provide(
		func(cell.Lifecycle, client.Clientset) (daemonk8s.LocalPodResource, error) {
			return &fakeresource[*slim_corev1.Pod]{}, nil
		},
		func() resource.Resource[*slim_corev1.Namespace] {
			return &fakeresource[*slim_corev1.Namespace]{}
		},
		func() daemonk8s.LocalNodeResource {
			return &fakeresource[*slim_corev1.Node]{}
		},
		func() daemonk8s.LocalCiliumNodeResource {
			return &fakeresource[*cilium_api_v2.CiliumNode]{}
		},
		func() resource.Resource[*slim_networkingv1.NetworkPolicy] {
			return &fakeresource[*slim_networkingv1.NetworkPolicy]{}
		},
		func() resource.Resource[*cilium_api_v2.CiliumNetworkPolicy] {
			return &fakeresource[*cilium_api_v2.CiliumNetworkPolicy]{}
		},
		func() resource.Resource[*cilium_api_v2.CiliumClusterwideNetworkPolicy] {
			return &fakeresource[*cilium_api_v2.CiliumClusterwideNetworkPolicy]{}
		},
		func() resource.Resource[*cilium_api_v2alpha1.CiliumCIDRGroup] {
			return &fakeresource[*cilium_api_v2alpha1.CiliumCIDRGroup]{}
		},
		func() resource.Resource[*cilium_api_v2alpha1.CiliumEndpointSlice] {
			return &fakeresource[*cilium_api_v2alpha1.CiliumEndpointSlice]{}
		},
		func() resource.Resource[*cilium_api_v2.CiliumNode] {
			return &fakeresource[*cilium_api_v2.CiliumNode]{}
		},
		func() watchers.WatcherConfiguration {
			return &watcherconfig{}
		},
	),

	metrics.Cell,

	endpointmanager.Cell,

	cell.Provide(func() *policy.Updater {
		return &policy.Updater{}
	}),

	cell.Provide(func() *redirectpolicy.Manager {
		return &redirectpolicy.Manager{}
	}),

	cell.Provide(func() types.BandwidthManager {
		return &fakeBandwidthManager{}
	}),

	cell.Provide(func() service.ServiceManager {
		return &service.Service{}
	}),

	cgmngr.Cell,

	// Provide the resources needed by the watchers.

	cell.Provide(func(lc cell.Lifecycle, cs client.Clientset) (resource.Resource[*k8sTypes.CiliumEndpoint], error) {
		return ciliumk8s.CiliumSlimEndpointResource(ciliumk8s.CiliumResourceParams{
			Lifecycle: lc,
			ClientSet: cs,
		}, nil, func(*metav1.ListOptions) {})
	}),

	cell.Provide(func(lc cell.Lifecycle, cs client.Clientset) (resource.Resource[*ciliumk8s.Endpoints], error) {
		//nolint:wrapcheck // a wrapped error here is of dubious value
		return ciliumk8s.EndpointsResource(lc, ciliumk8s.Config{
			EnableK8sEndpointSlice: true,
			K8sServiceProxyName:    "",
		}, cs)
	}),

	cell.Provide(func(lc cell.Lifecycle, cs client.Clientset) (resource.Resource[*slim_corev1.Service], error) {
		//nolint:wrapcheck // a wrapped error here is of dubious value
		return ciliumk8s.ServiceResource(
			lc,
			ciliumk8s.Config{
				EnableK8sEndpointSlice: false,
			},
			cs,
			func(*metav1.ListOptions) {},
		)
	}),

	// Provide everything needed for the watchers.
	cell.Provide(func() *ipcache.IPCache {
		iao := &identityAllocatorOwner{}
		idAlloc := &cachingIdentityAllocator{
			cache.NewCachingIdentityAllocator(iao),
			nil,
		}
		return ipcache.NewIPCache(&ipcache.Configuration{
			Context:               context.Background(),
			IdentityAllocator:     idAlloc,
			PolicyHandler:         &policyhandler{},
			DatapathHandler:       &datapathhandler{},
			DisableLabelInjection: true,
		})
	}),

	cell.Provide(func() node.LocalNodeSynchronizer {
		return &nodeSynchronizer{
			l: logrus.WithField("module", "node-synchronizer"),
		}
	}),

	cell.Provide(func() ipset.Manager {
		return &fakeIpsetMgr{}
	}),

	cell.Provide(func() speaker.MetalLBBgpSpeaker {
		return &fakeMetalLBBgpSpeaker{}
	}),

	manager.Cell,

	node.LocalNodeStoreCell,

	synced.Cell,
	cell.Provide(newAPIServerEventHandler),

	watchers.Cell,

	cell.Invoke(func(a *APIServerEventHandler) {
		ps := pubsub.New()
		fn := pubsub.CallBackFunc(a.handleAPIServerEvent)
		uuid := ps.Subscribe(common.PubSubAPIServer, &fn)
		a.l.WithFields(logrus.Fields{
			"uuid": uuid,
		}).Info("Subscribed to PubSub APIServer")
	}),
)
