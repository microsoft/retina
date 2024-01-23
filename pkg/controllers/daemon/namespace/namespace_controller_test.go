// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package namespacecontroller

import (
	"context"
	"testing"
	"time"

	retinav1alpha1 "github.com/microsoft/retina/crd/api/v1alpha1"
	"github.com/microsoft/retina/pkg/common"
	"github.com/microsoft/retina/pkg/controllers/cache"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/module/metrics"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

var fakescheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(fakescheme))
	utilruntime.Must(retinav1alpha1.AddToScheme(fakescheme))

	_ = clientgoscheme.AddToScheme(fakescheme)
	fakescheme.AddKnownTypes(retinav1alpha1.GroupVersion, &retinav1alpha1.RetinaEndpoint{})
}

func TestNamespaceControllerReconcile(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	type fields struct {
		existingObjects []client.Object
	}
	type args struct {
		ctx context.Context
		req ctrl.Request
	}
	tests := []struct {
		name        string
		fields      fields
		args        args
		want        ctrl.Result
		wantErr     bool
		deleteCalls int
		addCalls    int
	}{
		{
			name: "Test Namespace Controller Reconcile namespace not found (deleted)",
			fields: fields{
				existingObjects: []client.Object{},
			},
			args: args{
				req: ctrl.Request{
					NamespacedName: client.ObjectKey{
						Name: "test",
					},
				},
			},
			wantErr:     false,
			deleteCalls: 1,
			addCalls:    0,
		},
		{
			name: "Test Namespace Controller Reconcile with annotation (namespace updated/added with annotation)",
			fields: fields{
				existingObjects: []client.Object{
					&corev1.Namespace{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
							Annotations: map[string]string{
								common.RetinaPodAnnotation: common.RetinaPodAnnotationValue,
							},
						},
					},
				},
			},
			args: args{
				req: ctrl.Request{
					NamespacedName: client.ObjectKey{
						Name: "test",
					},
				},
			},
			wantErr:     false,
			deleteCalls: 0,
			addCalls:    1,
		},
		{
			name: "Test Namespace Controller Reconcile without annotation delete",
			fields: fields{
				existingObjects: []client.Object{
					&corev1.Namespace{
						ObjectMeta: metav1.ObjectMeta{
							Name:        "test",
							Annotations: map[string]string{},
						},
					},
				},
			},
			args: args{
				req: ctrl.Request{
					NamespacedName: client.ObjectKey{
						Name: "test",
					},
				},
			},
			wantErr:     false,
			deleteCalls: 1,
			addCalls:    0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientBuilder().WithScheme(fakescheme).WithObjects(tt.fields.existingObjects...).Build()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			cache := cache.NewMockCacheInterface(ctrl) //nolint:typecheck
			cache.EXPECT().AddAnnotatedNamespace(gomock.Any()).Return().Times(tt.addCalls)
			cache.EXPECT().DeleteAnnotatedNamespace(gomock.Any()).Return().Times(tt.deleteCalls)
			r := New(client, cache, &metrics.Module{})
			_, err := r.Reconcile(tt.args.ctx, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("NamespaceControllerReconciler.Reconcile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestPredicateFuncs(t *testing.T) {
	funcs := getPredicateFuncs()
	assert.True(t, funcs.Create(event.CreateEvent{
		Object: &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
				Annotations: map[string]string{
					common.RetinaPodAnnotation: common.RetinaPodAnnotationValue,
				},
			},
		},
	}))
	assert.True(t, funcs.Update(event.UpdateEvent{
		ObjectOld: &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
				Annotations: map[string]string{
					common.RetinaPodAnnotation: common.RetinaPodAnnotationValue,
				},
			},
		},
		ObjectNew: &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "test",
				Annotations: map[string]string{},
			},
		},
	}))
	assert.True(t, funcs.Delete(event.DeleteEvent{
		Object: &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
				Annotations: map[string]string{
					common.RetinaPodAnnotation: common.RetinaPodAnnotationValue,
				},
			},
		},
	}))
}

func TestStart(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	existingObjects := []client.Object{
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "test",
				Annotations: map[string]string{},
			},
		},
	}
	client := fake.NewClientBuilder().WithScheme(fakescheme).WithObjects(existingObjects...).Build()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	cache := cache.NewMockCacheInterface(ctrl) //nolint:typecheck
	cache.EXPECT().GetAnnotatedNamespaces().Return([]string{"test"}).MinTimes(1)
	mm := metrics.NewMockIModule(ctrl) //nolint:typecheck
	// metrics_module Reconcile only called 1 time since namespaces is not dirty after first call.
	mm.EXPECT().Reconcile(gomock.Any()).Return(nil).MinTimes(2)
	r := New(client, cache, mm)
	// add multiple reocncile calls to the channel to confirm reconcile is only called once
	ctx, cancel := context.WithCancel(context.Background())
	go r.Start(ctx)
	time.Sleep(15 * time.Second)
	cancel()
}
