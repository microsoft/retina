/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package podcontroller

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	retinav1alpha1 "github.com/microsoft/retina/crd/api/v1alpha1"
	"github.com/microsoft/retina/operator/cache"
)

var fakescheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(fakescheme))
	utilruntime.Must(retinav1alpha1.AddToScheme(fakescheme))

	_ = clientgoscheme.AddToScheme(fakescheme)
	fakescheme.AddKnownTypes(retinav1alpha1.GroupVersion, &retinav1alpha1.RetinaEndpoint{})
}

func TestPodReconciler_Reconcile(t *testing.T) {
	type fields struct {
		existingObjects []client.Object
	}
	type args struct {
		ctx context.Context
		req ctrl.Request
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    ctrl.Result
		wantErr bool
		wantNil bool
	}{
		{
			name: "test if pod is created",
			fields: fields{
				existingObjects: []client.Object{
					&corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-pod",
							Namespace: "test-ns",
						},
					},
				},
			},
			args: args{
				ctx: context.TODO(),
				req: ctrl.Request{
					NamespacedName: types.NamespacedName{
						Name:      "test-pod",
						Namespace: "test-ns",
					},
				},
			},
			wantNil: false,
		},
		{
			name: "test if pod is deleted",
			fields: fields{
				existingObjects: []client.Object{},
			},
			args: args{
				ctx: context.TODO(),
				req: ctrl.Request{
					NamespacedName: types.NamespacedName{
						Name:      "test-pod",
						Namespace: "test-ns",
					},
				},
			},
			wantNil: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientBuilder().WithScheme(fakescheme).WithObjects(tt.fields.existingObjects...).Build()
			podchannel := make(chan cache.PodCacheObject, 10)

			r := New(client, fakescheme, podchannel)

			got, err := r.Reconcile(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				require.Error(t, err)
			}
			require.Equal(t, got, tt.want)

			// verify that the pod is deleted
			select {
			case pod := <-podchannel:
				if tt.wantNil {
					require.Nil(t, pod.Pod)
				} else {
					require.NotNil(t, pod.Pod)
				}

			case <-time.After(5 * time.Second):
				require.Errorf(t, nil, "pod controller didn't write to channel after allotted time")
			}
		})
	}
}

func Test_isUpdatedPod(t *testing.T) {
	type args struct {
		old metav1.Object
		new metav1.Object
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "test if pod not is updated",
			args: args{
				old: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "test-ns",
					},
					Status: corev1.PodStatus{
						PodIP: "10.0.0.1",
					},
				},
				new: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "test-ns",
					},
					Status: corev1.PodStatus{
						PodIP: "10.0.0.1",
					},
				},
			},
			want: false,
		},
		{
			name: "test if pod is updated with single IP",
			args: args{
				old: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "test-ns",
					},
					Status: corev1.PodStatus{
						PodIP: "10.0.0.1",
					},
				},
				new: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "test-ns",
					},
					Status: corev1.PodStatus{
						PodIP: "10.0.0.2",
					},
				},
			},
			want: true,
		},
		{
			name: "test if pod is updated with IP slice",
			args: args{
				old: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "test-ns",
					},
					Status: corev1.PodStatus{
						PodIPs: []corev1.PodIP{
							{
								IP: "10.0.0.1",
							},
						},
					},
				},
				new: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "test-ns",
					},
					Status: corev1.PodStatus{
						PodIPs: []corev1.PodIP{
							{
								IP: "10.0.0.2",
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "test if pod is updated with labels",
			args: args{
				old: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "test-ns",
						Labels: map[string]string{
							"test": "test",
						},
					},
				},
				new: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "test-ns",
						Labels: map[string]string{
							"test": "test2",
						},
					},
				},
			},
			want: true,
		},
		{
			name: "test if pod is updated with annotations",
			args: args{
				old: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "test-ns",
						Annotations: map[string]string{
							"test": "test",
						},
					},
				},
				new: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "test-ns",
						Annotations: map[string]string{
							"test": "test2",
						},
					},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := filterPodUpdateEvents(tt.args.old, tt.args.new); got != tt.want {
				t.Errorf("isUpdatedPod() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterPodCreateEvents(t *testing.T) {
	type args struct {
		objMeta metav1.Object
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// TODO: Add test cases.
		{
			name: "test if pod is scheduled but not running",
			args: args{
				objMeta: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "test-ns",
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodPhase(corev1.PodScheduled),
					},
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := filterPodCreateEvents(tt.args.objMeta); got != tt.want {
				t.Errorf("filterPodCreateEvents() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterPodUpdateEvents(t *testing.T) {
	type args struct {
		old metav1.Object
		new metav1.Object
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "test host network pod",
			args: args{
				old: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "test-ns",
					},
					Spec: corev1.PodSpec{
						HostNetwork: true,
					},
				},
				new: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "test-ns",
					},
				},
			},
			want: false,
		},
		{
			name: "test if pod not is updated",
			args: args{
				old: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "test-ns",
					},
					Status: corev1.PodStatus{
						PodIP: "10.0.0.41",
					},
				},
				new: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "test-ns",
					},
					Status: corev1.PodStatus{
						PodIP: "10.0.0.4",
					},
				},
			},
			want: true,
		},
		{
			name: "test if pod is updated with single IP",
			args: args{
				old: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "test-ns",
					},
					Status: corev1.PodStatus{
						PodIP: "10.0.0.0",
					},
				},
				new: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "test-ns",
					},
					Status: corev1.PodStatus{
						PodIP: "10.0.0.1",
					},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		if got := filterPodUpdateEvents(tt.args.old, tt.args.new); got != tt.want {
			t.Errorf("filterPodUpdateEvents() = %v, want %v", got, tt.want)
		}
	}
}

func TestGetContainers(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-ns",
		},
		Status: corev1.PodStatus{
			PodIP: "10.0.0.0",
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:        "test-container",
					ContainerID: "123",
				},
				{
					Name:        "test-container2",
					ContainerID: "1234",
				},
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "test-container",
				},
			},
		},
	}

	kesc := getContainers(pod)

	assert.Equal(t, 2, len(kesc), "should have 2 containers")
	assert.Equal(t, "test-container", kesc[0].Name, "should have test-container")

	assert.Equal(t, "123", kesc[0].ID, "should have test-container")
	assert.Equal(t, "test-container2", kesc[1].Name, "should have test-container2")
	assert.Equal(t, "1234", kesc[1].ID, "should have test-container")
}
