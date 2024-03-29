package cache

import (
	"sync"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	retinav1alpha1 "github.com/microsoft/retina/crd/api/v1alpha1"
)

type PodCacheObject struct {
	Key types.NamespacedName // cache.MetaNamespaceKeyFunc
	Pod *corev1.Pod
}

type MetricsConfigurationCacheObject struct {
	Key                  string // cache.MetaNamespaceKeyFunc
	MetricsConfiguration *retinav1alpha1.MetricsConfiguration
}

type TraceConfigurationCacheObject struct {
	Key                string // cache.MetaNamespaceKeyFunc
	TraceConfiguration *retinav1alpha1.TraceConfiguration
}

type RetinaEndpoint struct {
	sync.RWMutex
	Name       string
	Namespace  string
	OwnerRefs  []metav1.OwnerReference
	IPv4       string
	IPv6       string
	OtherIPv4s []string
	OtherIPv6s []string
	Containers []RetinaContainer
	Labels     map[string]string
}

type RetinaContainer struct {
	Name string
	ID   string
}

type RetinaSvc struct {
	sync.RWMutex
	Name       string
	Namespace  string
	ClusterIP  string
	ClusterIPs []string
	LBIP       string
	Selector   map[string]string
}
