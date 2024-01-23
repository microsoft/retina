// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
//go:build unit
// +build unit

package cache

import (
	"context"
	"testing"

	"github.com/microsoft/retina/pkg/log"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	v1 "k8s.io/client-go/informers/core/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	crc "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crmgr "sigs.k8s.io/controller-runtime/pkg/manager"
)

type mockMgr struct {
	crmgr.Manager
}

type mockCache struct {
	crc.Cache
}

type mockClient struct {
	client.Client
}

func (mm *mockMgr) GetCache() crc.Cache {
	var mc crc.Cache = &mockCache{}
	return mc
}

func (mm *mockMgr) GetClient() client.Client {
	var mc client.Client = &mockClient{}
	return mc
}

func (mc *mockClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	obj.SetOwnerReferences([]metav1.OwnerReference{
		{
			Name: "Test",
		},
	})
	return nil
}

func (mm *mockMgr) GetConfig() *rest.Config {
	return &rest.Config{}
}

func (mc *mockCache) Start(ctx context.Context) error {
	return nil
}

func (mc *mockCache) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	podItems := make([]corev1.Pod, 1)
	podItems[0] = corev1.Pod{
		Status: corev1.PodStatus{
			PodIP: "0.0.0.1",
		},
	}
	svcItems := make([]corev1.Service, 1)
	svcItems[0] = corev1.Service{
		Spec: corev1.ServiceSpec{
			ClusterIP: "0.0.0.2",
		},
	}
	switch list.(type) {
	case *corev1.PodList:
		listPtr := (list).(*corev1.PodList)
		listPtr.Items = podItems
	case *corev1.ServiceList:
		listPtr := (list).(*corev1.ServiceList)
		listPtr.Items = svcItems
	}
	return nil
}

func TestCache_LookupObjectByIP(t *testing.T) {
	type fields struct {
		objCache    map[string]client.Object
		podInformer v1.PodInformer
		svcInformer v1.ServiceInformer
	}
	type args struct {
		ip string
	}
	var mm crmgr.Manager = &mockMgr{}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantIP  string
		wantErr bool
	}{
		{
			name: "Pod in local cache",
			fields: fields{
				objCache: map[string]client.Object{
					"0.0.0.01": &corev1.Pod{
						Status: corev1.PodStatus{
							PodIP: "0.0.0.01",
						},
					},
				},
			},
			args: args{
				ip: "0.0.0.01",
			},
			wantIP:  "0.0.0.01",
			wantErr: false,
		},
		{
			name: "Pod in cache",
			fields: fields{
				objCache: map[string]client.Object{},
			},
			args: args{
				ip: "0.0.0.1",
			},
			wantIP:  "0.0.0.1",
			wantErr: false,
		},
		{
			name: "Service in cache",
			fields: fields{
				objCache: map[string]client.Object{},
			},
			args: args{
				ip: "0.0.0.2",
			},
			wantIP:  "0.0.0.2",
			wantErr: false,
		},
		{
			name: "Pod/Svc not found",
			fields: fields{
				objCache: map[string]client.Object{},
			},
			args: args{
				ip: "0.0.0.3",
			},
			wantIP:  "0.0.0.3",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Cache{
				objCache:    tt.fields.objCache,
				podInformer: tt.fields.podInformer,
				svcInformer: tt.fields.svcInformer,
				mgr:         mm,
			}
			c.SetInformers(informers.NewSharedInformerFactory(nil, 0))
			got, err := c.LookupObjectByIP(tt.args.ip)
			if err != nil && tt.wantErr {
				return
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("Cache.LookupObjectByIP() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			switch v := got.(type) {
			case *corev1.Pod:
				if v.Status.PodIP != tt.wantIP {
					t.Errorf("Cache.LookupObjectByIP() = %v, want %v", got, tt.wantIP)
				}
			case *corev1.Service:
				if v.Spec.ClusterIP != tt.wantIP {
					t.Errorf("Cache.LookupObjectByIP() = %v, want %v", got, tt.wantIP)
				}
			}
		})
	}
}

func TestCache_update(t *testing.T) {
	type fields struct {
		objCache    map[string]client.Object
		podInformer v1.PodInformer
		svcInformer v1.ServiceInformer
	}
	type args struct {
		old interface{}
		new interface{}
	}
	cObj := corev1.Pod{
		Status: corev1.PodStatus{
			PodIP: "0.0.0.01",
		},
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "Test update for object cache where ip > 0",
			fields: fields{
				objCache: map[string]client.Object{
					"0.0.0.01": &cObj,
				},
			},
			args: args{
				old: &cObj,
				new: &cObj,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Cache{
				objCache:    tt.fields.objCache,
				podInformer: tt.fields.podInformer,
				svcInformer: tt.fields.svcInformer,
			}
			c.update(tt.args.old, tt.args.new)
		})
	}
}

func TestCache_delete(t *testing.T) {
	type fields struct {
		objCache    map[string]client.Object
		podInformer v1.PodInformer
		svcInformer v1.ServiceInformer
	}
	type args struct {
		obj interface{}
	}
	cObj := corev1.Pod{
		Status: corev1.PodStatus{
			PodIP: "0.0.0.01",
		},
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "Test update for object cache where ip > 0",
			fields: fields{
				objCache: map[string]client.Object{
					"0.0.0.01": &cObj,
				},
			},
			args: args{
				obj: &cObj,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Cache{
				objCache:    tt.fields.objCache,
				podInformer: tt.fields.podInformer,
				svcInformer: tt.fields.svcInformer,
			}
			c.delete(tt.args.obj)
			if c.objCache[cObj.Status.PodIP] != nil {
				t.Errorf("Cache.delete() = cache entry was not deleted %v", cObj.Status.PodIP)
			}
		})
	}
}

func TestStart(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "Test new cache",
			wantErr: false,
		},
	}
	cl := k8sfake.NewSimpleClientset()
	factory := informers.NewSharedInformerFactory(cl, ResyncTime)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log.SetupZapLogger(log.GetDefaultLogOpts())
			c := New(log.Logger(), &mockMgr{}, cl, factory)
			err := c.Start(context.TODO())
			if err != nil {
				t.Errorf("Error starting the cache %v", err)
			}
		})
	}
}

func TestGetPodOwner(t *testing.T) {
	type args struct {
		obj interface{}
	}
	tests := []struct {
		name      string
		args      args
		ownerName string
		ownerKind string
	}{
		{
			name: "Test daemonset",
			args: args{
				obj: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind: "DaemonSet",
								Name: "Test",
							},
						},
					},
				},
			},
			ownerName: "Test",
			ownerKind: "DaemonSet",
		},
		{
			name: "Test deployment",
			args: args{
				obj: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind: "ReplicaSet",
								Name: "Test",
							},
						},
					},
				},
			},
			ownerName: "Test",
			ownerKind: "Deployment",
		},
		{
			name: "Test unkown owner",
			args: args{
				obj: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{},
					},
				},
			},
		},
	}
	mm := &mockMgr{}
	c := &Cache{
		mgr: mm,
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got1, got2 := c.GetPodOwner(tt.args.obj)
			if got1 != tt.ownerName || got2 != tt.ownerKind {
				t.Errorf("GetPodOwner() got1 = %v, want1 %v, got2 = %v, want2: %v", got1, tt.ownerName, got2, tt.ownerKind)
			}
		})
	}
}
