package cache

import (
	slim_corev1 "github.com/cilium/cilium/pkg/k8s/slim/k8s/api/core/v1"

	"github.com/cilium/cilium/pkg/k8s/resource"
)

type PodCacheObject struct {
	Key resource.Key
	Pod *slim_corev1.Pod
}
