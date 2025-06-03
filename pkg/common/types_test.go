// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package common

import (
	"net"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	retinav1alpha1 "github.com/microsoft/retina/crd/api/v1alpha1"
)

func TestRetinaEndpointCommonFromAPI(t *testing.T) {
	ownerReference := retinav1alpha1.OwnerReference{
		APIVersion: "apps/v1",
		Kind:       "DaemonSet",
		Name:       "retina-agent",
	}

	hostIP := "10.224.0.253"
	podIP := "10.0.0.1"
	podIPv6 := "2001:1234:5678:9abc::4"
	parsedPodIP := net.ParseIP(podIP)
	parsedPodIPV6 := net.ParseIP(podIPv6)

	container := retinav1alpha1.RetinaEndpointStatusContainers{
		Name: "test",
		ID:   "5d4afda7-490b-43c3-bcfa-ed8f5d23720e",
	}

	tests := []struct {
		name                     string
		retinaendpoint           *retinav1alpha1.RetinaEndpoint
		wantRetinaendpointCommon *RetinaEndpoint
	}{
		{
			name: "pod has one ip address",
			retinaendpoint: &retinav1alpha1.RetinaEndpoint{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: retinav1alpha1.RetinaEndpointSpec{
					Containers: []retinav1alpha1.RetinaEndpointStatusContainers{
						container,
					},
					Labels: map[string]string{"test": "test"},
					OwnerReferences: []retinav1alpha1.OwnerReference{
						ownerReference,
					},
					NodeIP: hostIP,
					PodIP:  podIP,
					PodIPs: []string{podIP},
					Annotations: map[string]string{
						RetinaPodAnnotation: RetinaPodAnnotationValue,
					},
				},
			},
			wantRetinaendpointCommon: &RetinaEndpoint{
				BaseObject: BaseObject{
					name:      "test",
					namespace: "test",
					ips: &IPAddresses{
						IPv4:       parsedPodIP,
						IPv6:       net.IP(""),
						OtherIPv4s: []net.IP{},
						OtherIPv6s: []net.IP{},
					},
				},
				ownerRefs: []*OwnerReference{
					{
						APIVersion: ownerReference.APIVersion,
						Kind:       ownerReference.Kind,
						Name:       ownerReference.Name,
					},
				},
				containers: []*RetinaContainer{
					{
						Name: container.Name,
						ID:   container.ID,
					},
				},
				annotations: map[string]string{
					RetinaPodAnnotation: RetinaPodAnnotationValue,
				},
				zone: "zone-1",
			},
		},
		{
			name: "pod has dual-stack ip addresses",
			retinaendpoint: &retinav1alpha1.RetinaEndpoint{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: retinav1alpha1.RetinaEndpointSpec{
					Containers: []retinav1alpha1.RetinaEndpointStatusContainers{
						container,
					},
					Labels: map[string]string{"test": "test"},
					OwnerReferences: []retinav1alpha1.OwnerReference{
						ownerReference,
					},
					NodeIP: hostIP,
					PodIP:  podIP,
					PodIPs: []string{podIP, podIPv6},
					Annotations: map[string]string{
						"test":              "test",
						RetinaPodAnnotation: RetinaPodAnnotationValue,
					},
				},
			},
			wantRetinaendpointCommon: &RetinaEndpoint{
				BaseObject: BaseObject{
					name:      "test",
					namespace: "test",
					ips: &IPAddresses{
						IPv4:       parsedPodIP,
						IPv6:       net.IP(""),
						OtherIPv4s: []net.IP{},
						OtherIPv6s: []net.IP{parsedPodIPV6},
					},
				},
				ownerRefs: []*OwnerReference{
					{
						APIVersion: ownerReference.APIVersion,
						Kind:       ownerReference.Kind,
						Name:       ownerReference.Name,
					},
				},
				containers: []*RetinaContainer{
					{
						Name: container.Name,
						ID:   container.ID,
					},
				},
				annotations: map[string]string{
					RetinaPodAnnotation: RetinaPodAnnotationValue,
				},
				zone: "zone-1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRetinaendpointCommon := RetinaEndpointCommonFromAPI(tt.retinaendpoint, "zone-1")

			if diff := cmp.Diff(gotRetinaendpointCommon, tt.wantRetinaendpointCommon, cmpopts.IgnoreFields(BaseObject{}, "RWMutex"), cmp.AllowUnexported(BaseObject{}, RetinaEndpoint{})); diff != "" {
				t.Fatalf("RetinaEndpointCommonFromAPI mismatch (-got, +want)\n%s", diff)
			}
		})
	}
}

func TestRetinaEndpointCommonFromPod(t *testing.T) {
	ownerReference := metav1.OwnerReference{
		APIVersion: "apps/v1",
		Kind:       "DaemonSet",
		Name:       "retina-agent",
	}

	podIP := "10.0.0.1"
	parsedPodIP := net.ParseIP(podIP)

	container := corev1.ContainerStatus{
		Name:        "test",
		ContainerID: "5d4afda7-490b-43c3-bcfa-ed8f5d23720e",
	}

	tests := []struct {
		name                     string
		pod                      *corev1.Pod
		wantRetinaendpointCommon *RetinaEndpoint
	}{
		{
			name: "pod retina endpoint",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
					OwnerReferences: []metav1.OwnerReference{
						ownerReference,
					},
					Annotations: map[string]string{
						RetinaPodAnnotation: RetinaPodAnnotationValue,
						"test":              "test",
					},
					Labels: map[string]string{
						"test": "test",
					},
				},
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						container,
					},
					PodIP: podIP,
					PodIPs: []corev1.PodIP{
						{
							IP: podIP,
						},
					},
				},
			},
			wantRetinaendpointCommon: &RetinaEndpoint{
				BaseObject: BaseObject{
					name:      "test",
					namespace: "test",
					ips: &IPAddresses{
						IPv4:       parsedPodIP,
						IPv6:       net.IP(""),
						OtherIPv4s: []net.IP{},
						OtherIPv6s: []net.IP{},
					},
				},
				ownerRefs: []*OwnerReference{
					{
						APIVersion: ownerReference.APIVersion,
						Kind:       ownerReference.Kind,
						Name:       ownerReference.Name,
					},
				},
				containers: []*RetinaContainer{
					{
						Name: container.Name,
						ID:   container.ContainerID,
					},
				},
				annotations: map[string]string{
					RetinaPodAnnotation: RetinaPodAnnotationValue,
				},
				labels: map[string]string{
					"test": "test",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRetinaendpointCommon := RetinaEndpointCommonFromPod(tt.pod)

			if diff := cmp.Diff(gotRetinaendpointCommon, tt.wantRetinaendpointCommon, cmpopts.IgnoreFields(BaseObject{}, "RWMutex"), cmp.AllowUnexported(BaseObject{}, RetinaEndpoint{})); diff != "" {
				t.Fatalf("RetinaEndpointCommonFromPod mismatch (-got, +want)\n%s", diff)
			}
		})
	}
}
