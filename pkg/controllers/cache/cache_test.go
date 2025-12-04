// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package cache

import (
	"net"
	"testing"
	"time"

	"github.com/microsoft/retina/pkg/common"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/pubsub"
	"github.com/stretchr/testify/assert"
	gomock "go.uber.org/mock/gomock"
)

const (
	until = 1 * time.Millisecond
)

func TestNewCache(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	p := pubsub.NewMockPubSubInterface(ctrl)
	p.EXPECT().Subscribe(common.PubSubAPIServer, gomock.Any()).Times(1)
	c := New(p)
	assert.NotNil(t, c)
}

func TestCacheEndpoints(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	p := pubsub.NewMockPubSubInterface(ctrl)
	p.EXPECT().Publish(common.PubSubPods, gomock.Any()).Times(2)
	p.EXPECT().Subscribe(common.PubSubAPIServer, gomock.Any()).Times(1)
	c := New(p)
	assert.NotNil(t, c)

	addEndpoints := common.NewRetinaEndpoint("pod1", "ns1", nil)
	addEndpoints.SetLabels(map[string]string{
		"app": "app1",
	})
	addEndpoints.SetAnnotations(map[string]string{
		common.RetinaPodAnnotation: common.RetinaPodAnnotationValue,
	})

	err := c.UpdateRetinaEndpoint(addEndpoints)
	assert.Error(t, err)

	addEndpoints.SetIPs(&common.IPAddresses{
		IPv4:       net.IPv4(1, 2, 3, 4),
		OtherIPv4s: []net.IP{net.IPv4(1, 2, 3, 5)},
	})

	err = c.UpdateRetinaEndpoint(addEndpoints)
	assert.NoError(t, err)

	obj := c.GetObjByIP("1.2.3.4")
	assert.NotNil(t, obj)
	ep := obj.(*common.RetinaEndpoint)
	assert.Equal(t, ep.Name(), addEndpoints.Name())
	assert.Equal(t, ep.Namespace(), addEndpoints.Namespace())
	assert.Equal(t, ep.Labels()["app"], addEndpoints.Labels()["app"])
	assert.Equal(t, ep.Annotations()[common.RetinaPodAnnotation], addEndpoints.Annotations()[common.RetinaPodAnnotation])

	// normal get by PrimaryIP
	ep = c.GetPodByIP("1.2.3.4")
	assert.Equal(t, ep.Name(), addEndpoints.Name())
	assert.Equal(t, ep.Namespace(), addEndpoints.Namespace())
	assert.Equal(t, ep.Labels()["app"], addEndpoints.Labels()["app"])
	assert.Equal(t, ep.Annotations()[common.RetinaPodAnnotation], addEndpoints.Annotations()[common.RetinaPodAnnotation])

	// get by secondary IP
	obj = c.GetObjByIP("1.2.3.5")
	assert.NotNil(t, obj)
	ep = obj.(*common.RetinaEndpoint)
	assert.Equal(t, ep.Name(), addEndpoints.Name())
	assert.Equal(t, ep.Namespace(), addEndpoints.Namespace())
	assert.Equal(t, ep.Labels()["app"], addEndpoints.Labels()["app"])
	assert.Equal(t, ep.Annotations()[common.RetinaPodAnnotation], addEndpoints.Annotations()[common.RetinaPodAnnotation])

	// normal get by secondary IP
	ep = c.GetPodByIP("1.2.3.5")
	assert.Equal(t, ep.Name(), addEndpoints.Name())
	assert.Equal(t, ep.Namespace(), addEndpoints.Namespace())
	assert.Equal(t, ep.Labels()["app"], addEndpoints.Labels()["app"])
	assert.Equal(t, ep.Annotations()[common.RetinaPodAnnotation], addEndpoints.Annotations()[common.RetinaPodAnnotation])

	// delete
	err = c.DeleteRetinaEndpoint(addEndpoints.Key())
	assert.NoError(t, err)

	time.Sleep(until)
}

func TestCacheServices(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	p := pubsub.NewMockPubSubInterface(ctrl)
	p.EXPECT().Subscribe(common.PubSubAPIServer, gomock.Any()).Times(1)
	c := New(p)
	assert.NotNil(t, c)

	addSvc := common.NewRetinaSvc("svc1", "ns1", nil, nil, nil)

	p.EXPECT().Publish(gomock.Any(), gomock.Any()).Times(2)
	err := c.UpdateRetinaSvc(addSvc)
	assert.Error(t, err)

	addSvc.SetIPs(&common.IPAddresses{
		IPv4: net.IPv4(1, 2, 3, 4),
	})

	err = c.UpdateRetinaSvc(addSvc)
	assert.NoError(t, err)

	obj := c.GetObjByIP("1.2.3.4")
	assert.NotNil(t, obj)
	svc := obj.(*common.RetinaSvc)
	assert.Equal(t, addSvc.Name(), svc.Name())
	assert.Equal(t, addSvc.Namespace(), svc.Namespace())
	assert.Equal(t, addSvc.Selector(), svc.Selector())
	assert.Equal(t, addSvc.LBIP(), svc.LBIP())

	// normal get
	svc = c.GetSvcByIP("1.2.3.4")
	assert.Equal(t, addSvc.Name(), svc.Name())
	assert.Equal(t, addSvc.Namespace(), svc.Namespace())
	assert.Equal(t, addSvc.Selector(), svc.Selector())
	assert.Equal(t, addSvc.LBIP(), svc.LBIP())

	// delete
	err = c.DeleteRetinaSvc(addSvc.Key())
	assert.NoError(t, err)

	time.Sleep(until)
}

func TestCacheNodes(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	p := pubsub.NewMockPubSubInterface(ctrl)
	p.EXPECT().Subscribe(common.PubSubAPIServer, gomock.Any()).Times(1)
	c := New(p)
	assert.NotNil(t, c)

	addNode := common.NewRetinaNode("node1", net.IPv4(1, 2, 3, 4), "zone-1")

	p.EXPECT().Publish(gomock.Any(), gomock.Any()).Times(2)
	err := c.UpdateRetinaNode(addNode)
	assert.NoError(t, err)

	obj := c.GetObjByIP("1.2.3.4")
	assert.NotNil(t, obj)
	node := obj.(*common.RetinaNode)
	assert.Equal(t, addNode.Name(), node.Name())
	assert.Equal(t, addNode.IPString(), node.IPString())
	assert.Equal(t, addNode.Zone(), node.Zone())

	// normal get
	node = c.GetNodeByIP("1.2.3.4")
	assert.Equal(t, addNode.Name(), node.Name())
	assert.Equal(t, addNode.IPString(), node.IPString())
	assert.Equal(t, addNode.Zone(), node.Zone())

	// delete
	err = c.DeleteRetinaNode(addNode.Name())
	assert.NoError(t, err)

	time.Sleep(until)
}

func TestAddPodSvcNodeSameIP(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	p := pubsub.NewMockPubSubInterface(ctrl)
	p.EXPECT().Publish(common.PubSubPods, gomock.Any()).Times(2)
	p.EXPECT().Publish(common.PubSubSvc, gomock.Any()).Times(2)
	p.EXPECT().Publish(common.PubSubNode, gomock.Any()).Times(1)
	p.EXPECT().Subscribe(common.PubSubAPIServer, gomock.Any()).Times(1)
	c := New(p)
	assert.NotNil(t, c)

	addEndpoints := common.NewRetinaEndpoint("pod1", "ns1", nil)
	addEndpoints.SetLabels(map[string]string{
		"app": "app1",
	})

	addEndpoints.SetIPs(&common.IPAddresses{
		IPv4: net.IPv4(1, 2, 3, 4),
	})

	err := c.UpdateRetinaEndpoint(addEndpoints)
	assert.NoError(t, err)

	addSvc := common.NewRetinaSvc("svc1", "ns1",
		&common.IPAddresses{
			IPv4: net.IPv4(1, 2, 3, 4),
		}, nil, nil)

	err = c.UpdateRetinaSvc(addSvc)
	assert.NoError(t, err)

	obj := c.GetObjByIP("1.2.3.4")
	assert.NotNil(t, obj)
	svc := obj.(*common.RetinaSvc)
	assert.Equal(t, addSvc.Name(), svc.Name())
	assert.Equal(t, addSvc.Namespace(), svc.Namespace())

	addNode := common.NewRetinaNode("node1", net.IPv4(1, 2, 3, 4), "zone-1")

	err = c.UpdateRetinaNode(addNode)
	assert.NoError(t, err)

	obj = c.GetObjByIP("1.2.3.4")
	assert.NotNil(t, obj)
	node := obj.(*common.RetinaNode)
	assert.Equal(t, addNode.Name(), node.Name())
	assert.Equal(t, addNode.IPString(), node.IPString())
	assert.Equal(t, addNode.Zone(), node.Zone())

	time.Sleep(until)
}

func TestAddPodSvcNodeSameIPDiffNS(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	p := pubsub.NewMockPubSubInterface(ctrl)
	p.EXPECT().Publish(common.PubSubPods, gomock.Any()).Times(2)
	p.EXPECT().Publish(common.PubSubSvc, gomock.Any()).Times(2)
	p.EXPECT().Publish(common.PubSubNode, gomock.Any()).Times(1)
	p.EXPECT().Subscribe(common.PubSubAPIServer, gomock.Any()).Times(1)
	c := New(p)
	assert.NotNil(t, c)

	addEndpoints := common.NewRetinaEndpoint("pod1", "ns1", nil)
	addEndpoints.SetLabels(map[string]string{
		"app": "app1",
	})

	addEndpoints.SetIPs(&common.IPAddresses{
		IPv4: net.IPv4(1, 2, 3, 4),
	})

	err := c.UpdateRetinaEndpoint(addEndpoints)
	assert.NoError(t, err)

	addSvc := common.NewRetinaSvc("svc1", "ns2",
		&common.IPAddresses{
			IPv4: net.IPv4(1, 2, 3, 4),
		}, nil, nil)

	err = c.UpdateRetinaSvc(addSvc)
	assert.NoError(t, err)

	obj := c.GetObjByIP("1.2.3.4")
	assert.NotNil(t, obj)
	svc := obj.(*common.RetinaSvc)
	assert.Equal(t, addSvc.Name(), svc.Name())
	assert.Equal(t, addSvc.Namespace(), svc.Namespace())

	addNode := common.NewRetinaNode("node1", net.IPv4(1, 2, 3, 4), "zone-1")

	err = c.UpdateRetinaNode(addNode)
	assert.NoError(t, err)

	obj = c.GetObjByIP("1.2.3.4")

	assert.NotNil(t, obj)
	node := obj.(*common.RetinaNode)
	assert.Equal(t, addNode.Name(), node.Name())
	assert.Equal(t, addNode.IPString(), node.IPString())
	assert.Equal(t, addNode.Zone(), node.Zone())

	time.Sleep(until)
}

func TestAddPodDiffNs(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	p := pubsub.NewMockPubSubInterface(ctrl)
	p.EXPECT().Publish(common.PubSubPods, gomock.Any()).Times(3)
	p.EXPECT().Subscribe(common.PubSubAPIServer, gomock.Any()).Times(1)
	c := New(p)
	assert.NotNil(t, c)

	addEndpoints := common.NewRetinaEndpoint("pod1", "ns1", nil)
	addEndpoints.SetLabels(map[string]string{
		"app": "app1",
	})

	addEndpoints.SetIPs(&common.IPAddresses{
		IPv4: net.IPv4(1, 2, 3, 4),
	})

	err := c.UpdateRetinaEndpoint(addEndpoints)
	assert.NoError(t, err)

	addEndpoints = common.NewRetinaEndpoint("pod1", "ns2", nil)
	addEndpoints.SetLabels(map[string]string{
		"app": "app1",
	})

	addEndpoints.SetIPs(&common.IPAddresses{
		IPv4: net.IPv4(1, 2, 3, 4),
	})

	err = c.UpdateRetinaEndpoint(addEndpoints)
	assert.NoError(t, err)

	obj := c.GetObjByIP("1.2.3.4")

	assert.NotNil(t, obj)
	ep := obj.(*common.RetinaEndpoint)
	assert.Equal(t, addEndpoints.Name(), ep.Name())
	assert.Equal(t, addEndpoints.Namespace(), ep.Namespace())

	time.Sleep(until)
}

func TestFailDelete(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	p := pubsub.NewMockPubSubInterface(ctrl)
	p.EXPECT().Subscribe(common.PubSubAPIServer, gomock.Any()).Times(1)
	c := New(p)
	assert.NotNil(t, c)

	addEndpoints := common.NewRetinaEndpoint("pod1", "ns1", nil)

	// Delete non-existing retina endpoint returns no error.
	err := c.DeleteRetinaEndpoint(addEndpoints.Key())
	assert.NoError(t, err)

	svc := common.NewRetinaSvc("svc1", "ns1", nil, nil, nil)
	err = c.DeleteRetinaSvc(svc.Key())
	assert.Error(t, err)

	node := common.NewRetinaNode("node1", net.IPv4(1, 2, 3, 4), "zone-1")

	err = c.DeleteRetinaNode(node.Name())
	assert.Error(t, err)

	time.Sleep(until)
}

func TestCachingNamespace(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	p := pubsub.NewMockPubSubInterface(ctrl)
	p.EXPECT().Subscribe(common.PubSubAPIServer, gomock.Any()).Times(1)
	c := New(p)
	ns := "test-ns"

	c.AddAnnotatedNamespace(ns)
	namespaces := c.GetAnnotatedNamespaces()
	assert.Equal(t, 1, len(namespaces))
	assert.Equal(t, ns, namespaces[0])
	c.DeleteAnnotatedNamespace(ns)
	namespaces = c.GetAnnotatedNamespaces()
	assert.Equal(t, 0, len(namespaces))
}
