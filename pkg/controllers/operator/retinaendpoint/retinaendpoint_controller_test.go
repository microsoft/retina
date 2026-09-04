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
	"github.com/microsoft/retina/pkg/log"
)

var fakescheme = runtime.NewScheme()

func TestRetinaEndpointReconciler_ReconcilePod(t *testing.T) {
	if _, err := log.SetupZapLogger(log.GetDefaultLogOpts()); err != nil {
		t.Errorf("Error setting up logger: %s", err)
	}
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
				// Wait for the RetinaEndpoint to be created/updated with the expected values
				require.Eventually(t, func() bool {
					err := client.Get(context.Background(), tt.fields.newlyCachedPod.Key, &got)
					if apierrors.IsNotFound(err) {
						return false
					}
					// Check that the spec matches what we expect (indicating create/update completed)
					return got.Spec.PodIP == tt.wantedRetinaEndpoint.Spec.PodIP
				}, 5*time.Second, 100*time.Millisecond, "RetinaEndpoint should be created/updated with expected PodIP")

				// Re-fetch to get the latest state
				err := client.Get(context.Background(), tt.fields.newlyCachedPod.Key, &got)
				require.NoError(t, err)

				// Compare the spec (ignore TypeMeta/ResourceVersion since fake client doesn't populate them)
				require.Equal(t, tt.wantedRetinaEndpoint.Spec, got.Spec)
				require.Equal(t, tt.wantedRetinaEndpoint.Name, got.Name)
				require.Equal(t, tt.wantedRetinaEndpoint.Namespace, got.Namespace)
			}
		})
	}
}

// podUID is the owning Pod's UID in the owner reference tests.
const podUID = types.UID("pod-uid-1")

func newOwnerRefTestReconciler(t *testing.T, objects ...client.Object) (*RetinaEndpointReconciler, client.Client) {
	t.Helper()
	if _, err := log.SetupZapLogger(log.GetDefaultLogOpts()); err != nil {
		t.Fatalf("Error setting up logger: %s", err)
	}
	utilruntime.Must(clientgoscheme.AddToScheme(fakescheme))
	utilruntime.Must(retinav1alpha1.AddToScheme(fakescheme))

	c := fake.NewClientBuilder().WithScheme(fakescheme).WithObjects(objects...).Build()
	return New(c, make(chan cache.PodCacheObject, 10)), c
}

func runningPodWithUID(name, namespace, podIP string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, UID: podUID},
		Status: corev1.PodStatus{
			Phase:  corev1.PodRunning,
			PodIP:  podIP,
			PodIPs: []corev1.PodIP{{IP: podIP}},
			HostIP: "10.10.10.10",
		},
	}
}

// requireOwnedByPod asserts the endpoint is garbage collectable via exactly one owner
// reference to the given Pod, without claiming the controller slot or blocking Pod
// deletion. Exactly one guards against references accumulating across reconciles.
func requireOwnedByPod(t *testing.T, endpoint *retinav1alpha1.RetinaEndpoint, name string, uid types.UID) {
	t.Helper()
	podRefs := []metav1.OwnerReference{}
	for _, ref := range endpoint.OwnerReferences {
		if ref.Kind == "Pod" {
			podRefs = append(podRefs, ref)
		}
	}
	require.Len(t, podRefs, 1, "expected exactly one Pod owner reference, got %v", endpoint.OwnerReferences)

	require.Equal(t, "v1", podRefs[0].APIVersion)
	require.Equal(t, name, podRefs[0].Name)
	require.Equal(t, uid, podRefs[0].UID)
	require.Nil(t, podRefs[0].Controller, "should not claim the controller reference")
	require.Nil(t, podRefs[0].BlockOwnerDeletion, "should not block Pod deletion")
}

func TestRetinaEndpointReconciler_CreateSetsPodOwnerReference(t *testing.T) {
	key := types.NamespacedName{Namespace: "default", Name: "pod"}
	pod := runningPodWithUID(key.Name, key.Namespace, "10.0.0.1")
	r, c := newOwnerRefTestReconciler(t, pod)

	require.NoError(t, r.reconcileRetinaEndpointFromPod(context.Background(), cache.PodCacheObject{Key: key, Pod: pod}))

	got := &retinav1alpha1.RetinaEndpoint{}
	require.NoError(t, c.Get(context.Background(), key, got))
	requireOwnedByPod(t, got, key.Name, podUID)

	// The informational spec field still mirrors the Pod's own owners, not the Pod.
	require.Empty(t, got.Spec.OwnerReferences)
}

func TestRetinaEndpointReconciler_UpdateAdoptsExistingEndpoint(t *testing.T) {
	key := types.NamespacedName{Namespace: "default", Name: "pod"}
	// An endpoint created before owner references were set carries none.
	existing := &retinav1alpha1.RetinaEndpoint{
		ObjectMeta: metav1.ObjectMeta{Name: key.Name, Namespace: key.Namespace},
		Spec:       retinav1alpha1.RetinaEndpointSpec{PodIP: "10.0.0.1"},
	}
	pod := runningPodWithUID(key.Name, key.Namespace, "10.0.0.2")
	r, c := newOwnerRefTestReconciler(t, existing, pod)

	require.NoError(t, r.reconcileRetinaEndpointFromPod(context.Background(), cache.PodCacheObject{Key: key, Pod: pod}))

	got := &retinav1alpha1.RetinaEndpoint{}
	require.NoError(t, c.Get(context.Background(), key, got))
	requireOwnedByPod(t, got, key.Name, podUID)
	require.Equal(t, "10.0.0.2", got.Spec.PodIP, "spec should still be updated")
}

func TestRetinaEndpointReconciler_UpdateReplacesStalePodOwnerReference(t *testing.T) {
	key := types.NamespacedName{Namespace: "default", Name: "pod"}
	// A Pod of the same name was recreated, so the endpoint points at a dead UID.
	staleUID := types.UID("pod-uid-old")
	existing := &retinav1alpha1.RetinaEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "v1",
				Kind:       "Pod",
				Name:       key.Name,
				UID:        staleUID,
			}},
		},
	}
	pod := runningPodWithUID(key.Name, key.Namespace, "10.0.0.3")
	r, c := newOwnerRefTestReconciler(t, existing, pod)

	require.NoError(t, r.reconcileRetinaEndpointFromPod(context.Background(), cache.PodCacheObject{Key: key, Pod: pod}))

	got := &retinav1alpha1.RetinaEndpoint{}
	require.NoError(t, c.Get(context.Background(), key, got))
	// requireOwnedByPod asserts exactly one Pod reference, so the stale UID is gone.
	requireOwnedByPod(t, got, key.Name, podUID)
}

func TestRetinaEndpointReconciler_SetPodOwnerRejectsCrossNamespace(t *testing.T) {
	r, _ := newOwnerRefTestReconciler(t)
	endpoint := &retinav1alpha1.RetinaEndpoint{
		ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "default"},
	}
	pod := runningPodWithUID("pod", "other", "10.0.0.1")

	require.Error(t, r.setPodOwner(endpoint, pod), "cross-namespace owner references are disallowed")
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
