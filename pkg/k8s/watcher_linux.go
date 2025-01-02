package k8s

import (
	"context"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/runtime"

	"github.com/cilium/cilium/pkg/k8s"
	"github.com/cilium/cilium/pkg/k8s/watchers"
	"github.com/cilium/cilium/pkg/logging"
	"github.com/cilium/cilium/pkg/logging/logfields"
	"github.com/cilium/cilium/pkg/option"
	"github.com/sirupsen/logrus"
)

func init() {
	// Register custom error handler for the watcher
	// nolint:reassign // this is the only way to set the error handler
	runtime.ErrorHandlers = []func(error){
		k8sWatcherErrorHandler,
	}
}

const (
	K8sAPIGroupCiliumEndpointV2 = "cilium/v2::CiliumEndpoint"
	K8sAPIGroupServiceV1Core    = "core/v1::Service"
)

var (
	logger       = logging.DefaultLogger.WithField(logfields.LogSubsys, "k8s-watcher")
	k8sResources = []string{K8sAPIGroupCiliumEndpointV2, K8sAPIGroupServiceV1Core}
)

func Start(ctx context.Context, k *watchers.K8sWatcher) {
	logger.Info("Starting Kubernetes watcher")

	option.Config.K8sSyncTimeout = 3 * time.Minute //nolint:gomnd // this duration is self-explanatory
	syncdCache := make(chan struct{})
	go k.InitK8sSubsystem(ctx, syncdCache)
	logger.WithField("k8s resources", k8sResources).Info("Kubernetes watcher started, will wait for cache sync")

	// Wait for K8s watcher to sync. If doesn't complete in 3 minutes, causes fatal error.
	<-syncdCache
	logger.Info("Kubernetes watcher synced")
}

// retinaK8sErrorHandler is a custom error handler for the watcher
// that logs the error and tags the error to easily identify
func k8sWatcherErrorHandler(e error) {
	errStr := e.Error()
	logError := func(er, r string) {
		logger.WithFields(logrus.Fields{
			"underlyingError": er,
			"resource":        r,
		}).Error("Error watching k8s resource")
	}

	switch {
	case strings.Contains(errStr, "Failed to watch *v1.Node"):
		logError(errStr, "v1.Node")
	case strings.Contains(errStr, "Failed to watch *v2.CiliumEndpoint"):
		logError(errStr, "v2.CiliumEndpoint")
	case strings.Contains(errStr, "Failed to watch *v1.Service"):
		logError(errStr, "v1.Service")
	case strings.Contains(errStr, "Failed to watch *v2.CiliumNode"):
		logError(errStr, "v2.CiliumNode")
	default:
		k8s.K8sErrorHandler(e)
	}
}
