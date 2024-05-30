// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package retinaendpointcontroller

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	retinav1alpha1 "github.com/microsoft/retina/crd/api/v1alpha1"
	"github.com/microsoft/retina/operator/cache"
)

var fakescheme = runtime.NewScheme()

func TestRetinaEndpointReconciler_ReconcilePod(t *testing.T) {
	utilruntime.Must(clientgoscheme.AddToScheme(fakescheme))
	utilruntime.Must(retinav1alpha1.AddToScheme(fakescheme))
	_ = clientgoscheme.AddToScheme(fakescheme)
	fakescheme.AddKnownTypes(retinav1alpha1.GroupVersion, &retinav1alpha1.RetinaEndpoint{})

	type fields struct {
		newlyCachedPod  cache.PodCacheObject
		existingObjects []client.Object
	}
	tests := []struct {
		name                 string
		fields               fields
		wantedRetinaEndpoint *retinav1alpha1.RetinaEndpoint
	}{
		{
			name: "update existing retina endpoint",
			fields: fields{
				existingObjects: []client.Object{
					&retinav1alpha1.RetinaEndpoint{
						TypeMeta: metav1.TypeMeta{
							Kind:       "RetinaEndpoint",
							APIVersion: "retina.sh/v1alpha1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "pod",
							Namespace: "default",
						},
						Spec: retinav1alpha1.RetinaEndpointSpec{
							PodIP: "10.0.0.1",
						},
					},
				},
				newlyCachedPod: cache.PodCacheObject{
					Key: types.NamespacedName{
						Name:      "pod",
						Namespace: "default",
					},
					Pod: &corev1.Pod{
						Status: corev1.PodStatus{
							PodIP: "10.0.0.2",
							Phase: corev1.PodRunning,
						},
						ObjectMeta: metav1.ObjectMeta{
							OwnerReferences: []metav1.OwnerReference{
								{
									Name: "pods",
									Kind: "Daemonset",
								},
							},
						},
					},
				},
			},
			wantedRetinaEndpoint: &retinav1alpha1.RetinaEndpoint{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "pod",
					Namespace:       "default",
					ResourceVersion: "1000",
				},
				Spec: retinav1alpha1.RetinaEndpointSpec{
					PodIP: "10.0.0.2",
					OwnerReferences: []retinav1alpha1.OwnerReference{
						{
							Name: "pods",
							Kind: "Daemonset",
						},
					},
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "RetinaEndpoint",
					APIVersion: "retina.sh/v1alpha1",
				},
			},
		},
		{
			name: "delete existing retina endpoint",
			fields: fields{
				existingObjects: []client.Object{
					&retinav1alpha1.RetinaEndpoint{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "pod",
							Namespace: "default",
						},
					},
				},
				newlyCachedPod: cache.PodCacheObject{
					Key: types.NamespacedName{
						Name:      "pod",
						Namespace: "default",
					},
					Pod: nil,
				},
			},
			wantedRetinaEndpoint: nil,
		},
		{
			name: "create retina endpoint from pod",
			fields: fields{
				newlyCachedPod: cache.PodCacheObject{
					Key: types.NamespacedName{
						Name:      "pod",
						Namespace: "default",
					},
					Pod: &corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "pod",
							Namespace: "default",
						},
						Status: corev1.PodStatus{
							Phase:  corev1.PodRunning,
							HostIP: "10.10.10.10",
							PodIPs: []corev1.PodIP{
								{
									IP: "10.0.0.2",
								},
							},
							PodIP: "10.0.0.1",
							ContainerStatuses: []corev1.ContainerStatus{
								{
									Name:        "testcontainer",
									ContainerID: "docker://1234567890",
								},
							},
						},
					},
				},
			},
			wantedRetinaEndpoint: &retinav1alpha1.RetinaEndpoint{
				TypeMeta: metav1.TypeMeta{
					Kind:       "RetinaEndpoint",
					APIVersion: "retina.sh/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            "pod",
					Namespace:       "default",
					ResourceVersion: "1",
				},
				Spec: retinav1alpha1.RetinaEndpointSpec{
					NodeIP: "10.10.10.10",
					PodIP:  "10.0.0.1",
					PodIPs: []string{"10.0.0.2"},
					Containers: []retinav1alpha1.RetinaEndpointStatusContainers{
						{
							Name: "testcontainer",
							ID:   "docker://1234567890",
						},
					},
				},
			},
		},
		{
			name: "create retina endpoint from non-running pod",
			fields: fields{
				newlyCachedPod: cache.PodCacheObject{
					Key: types.NamespacedName{
						Name:      "pod",
						Namespace: "default",
					},
					Pod: &corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "pod",
							Namespace: "default",
						},
						Status: corev1.PodStatus{
							Phase: corev1.PodPending,
						},
					},
				},
			},
			wantedRetinaEndpoint: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientBuilder().WithScheme(fakescheme).WithObjects(tt.fields.existingObjects...).Build()
			podchannel := make(chan cache.PodCacheObject, 10)
			r := New(client, podchannel)
			ctx, cancel := context.WithCancel(context.Background())
			go r.ReconcilePod(ctx)
			defer cancel()

			got := retinav1alpha1.RetinaEndpoint{}

			podchannel <- tt.fields.newlyCachedPod

			// Nil wantedRetinaEndpoint indicates no RetinaEndpoint is created from the newlyCachedPod.
			if tt.wantedRetinaEndpoint == nil {
				// No retinaEndpoint should be created consistently within timeout.
				require.Eventually(t, func() bool {
					err := client.Get(context.Background(), tt.fields.newlyCachedPod.Key, &got)
					return apierrors.IsNotFound(err)
				}, 5*time.Second, 1*time.Second, "RetinaEndpoint should not exist")
			} else {
				require.Eventually(t, func() bool {
					err := client.Get(context.Background(), tt.fields.newlyCachedPod.Key, &got)
					fmt.Println(err)
					return !apierrors.IsNotFound(err)
				}, 5*time.Second, 1*time.Second, "RetinaEndpoint should be created")
				require.Equal(t, *tt.wantedRetinaEndpoint, got)
			}
		})
	}
}

func TestRetinaEndpointReconciler_reqeuePodToRetinaEndpoint(t *testing.T) {
	type args struct {
		pod cache.PodCacheObject
	}
	tests := []struct {
		name     string
		args     args
		attempts int
	}{
		{
			name:     "requeue pod",
			attempts: 1,
			args: args{
				pod: cache.PodCacheObject{},
			},
		},

		{
			name:     "requeue pod twice",
			attempts: 2,
			args: args{
				pod: cache.PodCacheObject{},
			},
		},
		{
			name:     "more than max retries",
			attempts: MAX_RETRIES + 1,
			args: args{
				pod: cache.PodCacheObject{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientBuilder().WithScheme(fakescheme).Build()
			podchannel := make(chan cache.PodCacheObject, 10)

			r := New(client, podchannel)
			for i := 0; i < tt.attempts; i++ {
				r.requeuePodToRetinaEndpoint(context.Background(), tt.args.pod)
			}

			if tt.attempts >= MAX_RETRIES {
				require.Exactlyf(t, MAX_RETRIES, len(podchannel), fmt.Sprintf("podchannel length %d should be 0", len(podchannel)))
				require.Exactly(t, 0, len(r.retries), "retries should be empty")
			} else {
				require.Exactlyf(t, tt.attempts, len(podchannel), fmt.Sprintf("podchannel length %d should be %d", len(podchannel), tt.attempts))
				require.Exactlyf(t, tt.attempts, r.retries[tt.args.pod.Key], "retries should have %d entry", tt.attempts)
			}
		})
	}
}
