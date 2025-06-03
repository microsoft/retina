// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package common

import (
	"net"
	"sync"

	corev1 "k8s.io/api/core/v1"

	retinav1alpha1 "github.com/microsoft/retina/crd/api/v1alpha1"
)

const (
	APIServerEndpointName = "kubernetes-apiserver"
	// default value for annotations
	RetinaPodAnnotation      = "retina.sh"
	RetinaPodAnnotationValue = "observe"
)

// Important note: any changes to these structs must be reflected in the DeepCopy() method.

// PublishObj is an interface that all objects that are published
type PublishObj interface {
	DeepCopy() interface{}
}

// BaseObject is a common struct that is embedded in all objects that are published.
type BaseObject struct {
	*sync.RWMutex
	name      string
	namespace string
	ips       *IPAddresses
}

// RetinaEndpoint represents a Kubernetes endpoint.
type RetinaEndpoint struct {
	BaseObject
	ownerRefs   []*OwnerReference
	containers  []*RetinaContainer
	labels      map[string]string
	annotations map[string]string
}

func isIPV4(ipAddress string) bool {
	parsedIP := net.ParseIP(ipAddress)
	return parsedIP.To4() != nil
}

func RetinaEndpointCommonFromAPI(retinaEndpoint *retinav1alpha1.RetinaEndpoint) *RetinaEndpoint {
	retinaEndpointCommon := &RetinaEndpoint{
		BaseObject: BaseObject{
			name:      retinaEndpoint.Name,
			namespace: retinaEndpoint.Namespace,
			ips:       &IPAddresses{},
			RWMutex:   &sync.RWMutex{},
		},
		ownerRefs:   []*OwnerReference{},
		containers:  []*RetinaContainer{},
		labels:      retinaEndpoint.Labels,
		annotations: make(map[string]string),
	}

	for _, ownerRef := range retinaEndpoint.Spec.OwnerReferences {
		retinaEndpointCommon.ownerRefs = append(retinaEndpointCommon.ownerRefs, &OwnerReference{
			APIVersion: ownerRef.APIVersion,
			Kind:       ownerRef.Kind,
			Name:       ownerRef.Name,
		})
	}

	for _, container := range retinaEndpoint.Spec.Containers {
		retinaEndpointCommon.containers = append(retinaEndpointCommon.containers, &RetinaContainer{
			Name: container.Name,
			ID:   container.ID,
		})
	}

	podIP := net.ParseIP(retinaEndpoint.Spec.PodIP)
	if isIPV4(retinaEndpoint.Spec.PodIP) {
		retinaEndpointCommon.BaseObject.ips.IPv4 = podIP
	} else {
		retinaEndpointCommon.BaseObject.ips.IPv6 = podIP
	}

	OtherIPv4s := []net.IP{}
	OtherIPv6s := []net.IP{}
	for _, podIP := range retinaEndpoint.Spec.PodIPs {
		if podIP == retinaEndpoint.Spec.PodIP {
			continue
		}
		if isIPV4(podIP) {
			OtherIPv4s = append(OtherIPv4s, net.ParseIP(podIP))
			continue
		}
		OtherIPv6s = append(OtherIPv6s, net.ParseIP(podIP))
	}
	retinaEndpointCommon.ips.OtherIPv4s = OtherIPv4s
	retinaEndpointCommon.ips.OtherIPv6s = OtherIPv6s

	for k, v := range retinaEndpoint.Spec.Annotations {
		if k == RetinaPodAnnotation {
			retinaEndpointCommon.annotations[k] = v
		}
	}

	return retinaEndpointCommon
}

func RetinaEndpointCommonFromPod(pod *corev1.Pod) *RetinaEndpoint {
	retinaEndpointCommon := &RetinaEndpoint{
		BaseObject: BaseObject{
			name:      pod.Name,
			namespace: pod.Namespace,
			ips:       &IPAddresses{},
			RWMutex:   &sync.RWMutex{},
		},
		ownerRefs:   []*OwnerReference{},
		containers:  []*RetinaContainer{},
		labels:      pod.Labels,
		annotations: make(map[string]string),
	}

	for _, ownerRef := range pod.ObjectMeta.OwnerReferences {
		retinaEndpointCommon.ownerRefs = append(retinaEndpointCommon.ownerRefs, &OwnerReference{
			APIVersion: ownerRef.APIVersion,
			Kind:       ownerRef.Kind,
			Name:       ownerRef.Name,
		})
	}

	for _, container := range pod.Status.ContainerStatuses {
		retinaEndpointCommon.containers = append(retinaEndpointCommon.containers, &RetinaContainer{
			Name: container.Name,
			ID:   container.ContainerID,
		})
	}

	podIP := net.ParseIP(pod.Status.PodIP)
	if isIPV4(pod.Status.PodIP) {
		retinaEndpointCommon.BaseObject.ips.IPv4 = podIP
	} else {
		retinaEndpointCommon.BaseObject.ips.IPv6 = podIP
	}

	OtherIPv4s := []net.IP{}
	OtherIPv6s := []net.IP{}
	for _, podIP := range pod.Status.PodIPs {
		if podIP.IP == pod.Status.PodIP {
			continue
		}
		if isIPV4(podIP.IP) {
			OtherIPv4s = append(OtherIPv4s, net.ParseIP(podIP.IP))
			continue
		}
		OtherIPv6s = append(OtherIPv6s, net.ParseIP(podIP.IP))
	}
	retinaEndpointCommon.ips.OtherIPv4s = OtherIPv4s
	retinaEndpointCommon.ips.OtherIPv6s = OtherIPv6s

	for k, v := range pod.GetAnnotations() {
		if k == RetinaPodAnnotation {
			retinaEndpointCommon.annotations[k] = v
		}
	}

	return retinaEndpointCommon
}

// IPAddresses represents a set of IP addresses.
type IPAddresses struct {
	IPv4       net.IP
	IPv6       net.IP
	OtherIPv4s []net.IP
	OtherIPv6s []net.IP
}

// RetinaContainer represents a container in a Kubernetes endpoint.
type RetinaContainer struct {
	Name string
	ID   string
}

// RetinaSvc represents a Kubernetes service.
// ClusterIPs are saved as IPs in base object.
type RetinaSvc struct {
	BaseObject
	lbIP     net.IP
	selector map[string]string
}

// OwnerReference contains enough information to let you identify an owning object.
// Reference: https://github.com/kubernetes/apimachinery/blob/v0.26.4/pkg/apis/meta/v1/types.go#L291
type OwnerReference struct {
	APIVersion string
	Kind       string
	Name       string
	UID        string
	Controller bool
}

// RetinaNode represents a Kubernetes node.
type RetinaNode struct {
	name string
	ip   net.IP
	zone string
}

type APIServerObject struct {
	EP *RetinaEndpoint
}

func (a *APIServerObject) IPs() []net.IP {
	a.EP.RLock()
	defer a.EP.RUnlock()

	ips := []net.IP{}
	if a.EP.ips.IPv4 != nil {
		ips = append(ips, a.EP.ips.IPv4)
	}
	if a.EP.ips.OtherIPv4s != nil {
		ips = append(ips, a.EP.ips.OtherIPv4s...)
	}
	return ips
}

func NewAPIServerObject(stringIPs []string) *APIServerObject {
	ips := []net.IP{}
	for _, stringIP := range stringIPs {
		ip := net.ParseIP(stringIP)
		if ip != nil {
			ips = append(ips, ip)
		}
	}

	if len(ips) == 0 {
		return &APIServerObject{}
	}

	primaryIP := ips[0]
	ips = ips[1:]
	return &APIServerObject{
		EP: &RetinaEndpoint{
			BaseObject: BaseObject{
				name:      APIServerEndpointName,
				namespace: APIServerEndpointName,
				ips: &IPAddresses{
					IPv4:       primaryIP,
					OtherIPv4s: ips,
				},
				RWMutex: &sync.RWMutex{},
			},
		},
	}
}

func (a *APIServerObject) DeepCopy() interface{} {
	return &APIServerObject{
		EP: a.EP.DeepCopy().(*RetinaEndpoint),
	}
}
