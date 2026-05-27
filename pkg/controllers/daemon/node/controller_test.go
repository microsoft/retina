// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package node

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/microsoft/retina/pkg/common"
	"github.com/microsoft/retina/pkg/controllers/cache"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/pubsub"
	"github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReconcileUsesInternalIPWhenHostnameAddressIsFirst(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())

	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
			Labels: map[string]string{
				corev1.LabelTopologyZone: "zone-1",
			},
		},
		Status: corev1.NodeStatus{
			Addresses: []corev1.NodeAddress{
				{Type: corev1.NodeHostName, Address: "test-node"},
				{Type: corev1.NodeInternalIP, Address: "10.0.0.4"},
			},
		},
	}

	kubeClient := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&corev1.Node{}).WithObjects(node).Build()
	require.NoError(t, kubeClient.Status().Update(context.Background(), node))
	createdNode := &corev1.Node{}
	require.NoError(t, kubeClient.Get(context.Background(), client.ObjectKey{Name: node.Name}, createdNode))
	require.Len(t, createdNode.Status.Addresses, 2)

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	p := pubsub.NewMockPubSubInterface(mockCtrl)
	p.EXPECT().Subscribe(common.PubSubAPIServer, gomock.Any()).Times(1)
	published := make(chan struct{})
	var publishOnce sync.Once
	p.EXPECT().Publish(gomock.Any(), gomock.Any()).Do(func(pubsub.PubSubTopic, interface{}) {
		publishOnce.Do(func() {
			close(published)
		})
	}).AnyTimes()

	c := cache.New(p)
	r := New(kubeClient, c)

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKey{Name: node.Name},
	})
	require.NoError(t, err)

	select {
	case <-published:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for node cache publish")
	}

	cachedNode := c.GetNodeByIP("10.0.0.4")
	require.NotNil(t, cachedNode)
	require.Equal(t, node.Name, cachedNode.Name())
	require.Equal(t, "zone-1", cachedNode.Zone())
}
