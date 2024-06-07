package mock

import (
	"context"
	"errors"

	corev1 "k8s.io/api/core/v1"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Verify interface compliance at compile time
var _ kclient.Client = (*client)(nil)

type client struct {
	kclient.Client
	nodeCache map[string]*corev1.Node
}

// NewClient returns a new MockClient.
func NewClient() kclient.Client {
	return &client{
		nodeCache: make(map[string]*corev1.Node),
	}
}

// Get implements client.Client.Get.
func (c *client) Get(_ context.Context, key kclient.ObjectKey, obj kclient.Object, _ ...kclient.GetOption) error {
	node, ok := c.nodeCache[key.String()]
	if !ok {
		return kclient.IgnoreNotFound(errors.New("node not found"))
	}
	*obj.(*corev1.Node) = *node
	return nil
}

// Create implements client.Client.Create.
func (c *client) Create(_ context.Context, obj kclient.Object, _ ...kclient.CreateOption) error {
	node := obj.(*corev1.Node)
	c.nodeCache[node.Namespace+"/"+node.Name] = node
	return nil
}
