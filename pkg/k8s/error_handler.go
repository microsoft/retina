package k8s

import (
	"strings"

	"github.com/cilium/cilium/pkg/k8s"
)

// RetinaK8sErrorHandler is a wrapper around the k8s.K8sErrorHandler function
// that allows Retina to handle k8s errors that are more related to the usecase of this package.
func RetinaK8sErrorHandler(e error) {
	errStr := e.Error()
	switch {
	case strings.Contains(errStr, "Failed to watch *v1.Node"):
		logger.WithField("actualError", errStr).Error("Potentially Network Error coming from K8s API Server failing to watch Nodes")
	case strings.Contains(errStr, "Failed to watch *v2.CiliumEndpoint"):
		logger.WithField("actualError", errStr).Error("Potentially Network Error coming from K8s API Server failing to watch CiliumEndpoints")
	case strings.Contains(errStr, "Failed to watch *v1.Service"):
		logger.WithField("actualError", errStr).Error("Potentially Network Error coming from K8s API Server failing to watch Services")
	case strings.Contains(errStr, "Failed to watch *v2.CiliumNode"):
		logger.WithField("actualError", errStr).Error("Potentially Network Error coming from K8s API Server failing to watch CiliumNodes")

	default:
		k8s.K8sErrorHandler(e)
	}
}
