package k8s

import (
	"context"
	"sync"
	"time"

	agentK8s "github.com/cilium/cilium/daemon/k8s"
	"github.com/cilium/cilium/pkg/hive/cell"
	"github.com/cilium/cilium/pkg/ipcache"
	"github.com/cilium/cilium/pkg/k8s"
	"github.com/cilium/cilium/pkg/k8s/client"
	"github.com/cilium/cilium/pkg/k8s/synced"
	"github.com/cilium/cilium/pkg/k8s/watchers"
	"github.com/cilium/cilium/pkg/logging"
	"github.com/cilium/cilium/pkg/logging/logfields"
	"github.com/cilium/cilium/pkg/option"
)

const (
	K8sAPIGroupCiliumEndpointV2 = "cilium/v2::CiliumEndpoint"
)

var (
	once   sync.Once
	w      *watchers.K8sWatcher
	logger = logging.DefaultLogger.WithField(logfields.LogSubsys, "k8s-watcher")
	// k8sResources = []string{K8sAPIGroupCiliumEndpointV2, resources.K8sAPIGroupServiceV1Core}
	k8sResources = []string{}
)

type watcherParams struct {
	cell.In

	Lifecycle       cell.Lifecycle
	C               client.Clientset
	R               agentK8s.Resources
	IPcache         *ipcache.IPCache
	SvcCache        *k8s.ServiceCache
	Wcfg            watchers.WatcherConfiguration
	ResourcesSynced *synced.Resources
	APIGroups       *synced.APIGroups
}

func NewWatcher(params watcherParams) (*watchers.K8sWatcher, error) {
	return newInstance(params.C, params.ResourcesSynced, params.APIGroups, params.R, params.IPcache, params.SvcCache, params.Wcfg)
}

func newInstance(c client.Clientset, resourcesSynced *synced.Resources, apiGroups *synced.APIGroups, r agentK8s.Resources, ipc *ipcache.IPCache, svcCache *k8s.ServiceCache, wcfg watchers.WatcherConfiguration) (*watchers.K8sWatcher, error) {
	option.Config.BGPAnnounceLBIP = false
	once.Do(func() {
		w = watchers.NewK8sWatcher(
			c, // clientset
			resourcesSynced,
			apiGroups,
			&epmgr{},           // endpointManager
			&nodediscovermgr{}, // nodeDiscoverManager
			nil,                // policyManager
			nil,                // policyRepository
			nil,                // svcManager
			nil,                // Datapath
			nil,                // redirectPolicyManager
			nil,                // bgpSpeakerManager
			wcfg,               // WatcherConfiguration
			ipc,                // ipcacheManager
			&cgrpmgr{},         // cgroupManager
			r,                  // agentK8s.Resources
			svcCache,           // *k8s.ServiceCache
			nil,                // bandwidth.Manager
		)
	})
	return w, nil
}

func Start(ctx context.Context, k *watchers.K8sWatcher) {
	logger.Info("Starting Kubernetes watcher")

	option.Config.K8sSyncTimeout = 3 * time.Minute //nolint:gomnd // this duration is self-explanatory
	syncdCache := make(chan struct{})
	go k.InitK8sSubsystem(ctx, k8sResources, []string{}, syncdCache)
	logger.WithField("k8s resources", k8sResources).Info("Kubernetes watcher started, will wait for cache sync")

	// Wait for K8s watcher to sync. If doesn't complete in 3 minutes, causes fatal error.
	<-syncdCache
	logger.Info("Kubernetes watcher synced")
}
