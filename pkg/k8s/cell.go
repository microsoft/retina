package k8s

import (
	"context"

	daemonk8s "github.com/cilium/cilium/daemon/k8s"
	"github.com/cilium/cilium/pkg/hive/cell"
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
	"github.com/cilium/cilium/pkg/k8s/types"
	"github.com/cilium/cilium/pkg/k8s/watchers"
	"github.com/cilium/cilium/pkg/node"
	"github.com/cilium/cilium/pkg/option"
	"github.com/microsoft/retina/pkg/common"
	"github.com/microsoft/retina/pkg/pubsub"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var Cell = cell.Module(
	"k8s-watcher",
	"Kubernetes watchers needed by the agent",

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
		func() resource.Resource[*types.CiliumEndpoint] {
			return &fakeresource[*types.CiliumEndpoint]{}
		},
		func() resource.Resource[*cilium_api_v2.CiliumNode] {
			return &fakeresource[*cilium_api_v2.CiliumNode]{}
		},
		func() daemonk8s.ServiceNonHeadless {
			return &fakeresource[*slim_corev1.Service]{}
		},
		func() daemonk8s.EndpointsNonHeadless {
			return &fakeresource[*ciliumk8s.Endpoints]{}
		},
		func() watchers.WatcherConfiguration {
			return &watcherconfig{}
		},
	),

	cell.Provide(func(lc cell.Lifecycle, cs client.Clientset) (resource.Resource[*ciliumk8s.Endpoints], error) {
		return ciliumk8s.EndpointsResource(lc, ciliumk8s.Config{
			EnableK8sEndpointSlice: true,
			K8sServiceProxyName:    "",
		}, cs)
	}),

	cell.Provide(func(lc cell.Lifecycle, cs client.Clientset) (resource.Resource[*slim_corev1.Service], error) {
		return ciliumk8s.ServiceResource(
			lc,
			ciliumk8s.Config{
				EnableK8sEndpointSlice: false,
				K8sServiceProxyName:    "",
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
			Context:           context.Background(),
			IdentityAllocator: idAlloc,
			PolicyHandler:     &policyhandler{},
			DatapathHandler:   &datapathhandler{},
		})
	}),

	cell.Provide(func() *ciliumk8s.ServiceCache {
		option.Config.K8sServiceCacheSize = 1000
		return ciliumk8s.NewServiceCache(&nodeaddressing{})
	}),

	cell.Provide(func() node.LocalNodeSynchronizer {
		return &nodeSynchronizer{
			l: logrus.WithField("module", "node-synchronizer"),
		}
	}),
	node.LocalNodeStoreCell,

	synced.Cell,

	cell.Provide(NewWatcher),

	cell.Provide(newAPIServerEventHandler),
	cell.Invoke(func(a *ApiServerEventHandler) {
		ps := pubsub.New()
		fn := pubsub.CallBackFunc(a.handleAPIServerEvent)
		uuid := ps.Subscribe(common.PubSubAPIServer, &fn)
		a.l.WithFields(logrus.Fields{
			"uuid": uuid,
		}).Info("Subscribed to PubSub APIServer")
	}),
)
