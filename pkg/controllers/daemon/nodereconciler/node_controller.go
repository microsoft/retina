// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package nodereconciler

import (
	"context"
	"net"
	"reflect"
	"sync"

	"github.com/microsoft/retina/pkg/common/apiretry"
	"github.com/sirupsen/logrus"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	errors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	datapath "github.com/cilium/cilium/pkg/datapath/types"
	"github.com/cilium/cilium/pkg/identity"
	ipc "github.com/cilium/cilium/pkg/ipcache"
	"github.com/cilium/cilium/pkg/node/addressing"
	"github.com/cilium/cilium/pkg/node/types"
	"github.com/cilium/cilium/pkg/source"
)

// NodeReconciler reconciles a Node object.
// This is pretty basic for now, need fine tuning, scale test, etc.
type NodeReconciler struct {
	client.Client

	clusterName string

	l           logrus.FieldLogger
	handlers    map[string]datapath.NodeHandler
	nodes       map[string]types.Node
	c           *ipc.IPCache
	localNodeIP string
	m           sync.RWMutex
}

// isNodeUpdated checks if the node has been updated.
// This is a simple check for labels and annotations
// being updated. Those are the only fields that are mutable.
// AKS specific for now.
func isNodeUpdated(n1, n2 types.Node) bool {
	if !reflect.DeepEqual(n1.Labels, n2.Labels) {
		return true
	}
	if !reflect.DeepEqual(n1.Annotations, n2.Annotations) {
		return true
	}
	return false
}

func (r *NodeReconciler) addNode(node *corev1.Node) {
	r.m.Lock()
	defer r.m.Unlock()

	addresses := []types.Address{}
	for _, address := range node.Status.Addresses {
		if address.Type == corev1.NodeInternalIP {
			if ip := net.ParseIP(address.Address); ip != nil {
				addresses = append(addresses, types.Address{
					IP:   ip,
					Type: addressing.NodeInternalIP,
				})
			}
		}
		if address.Type == corev1.NodeExternalIP {
			if ip := net.ParseIP(address.Address); ip != nil {
				addresses = append(addresses, types.Address{
					IP:   ip,
					Type: addressing.NodeExternalIP,
				})
			}
		}
	}
	nd := types.Node{
		Name:        node.Name,
		IPAddresses: addresses,
		Labels:      node.Labels,
		Annotations: node.Annotations,
	}
	nd.Cluster = r.clusterName

	// Check if the node already exists.
	if curNode, ok := r.nodes[node.Name]; ok && !isNodeUpdated(curNode, nd) {
		r.l.Debug("Node already exists", zap.String("Node", node.Name))
		return
	}

	r.nodes[node.Name] = nd

	for _, handler := range r.handlers {
		err := handler.NodeAdd(nd)
		if err != nil {
			r.l.Error("Failed to add Node to datapath handler", zap.Error(err), zap.String("handler", handler.Name()), zap.String("Node", node.Name))
		}
	}

	id := identity.ReservedIdentityRemoteNode
	// Check if the node is the local node.
	for _, address := range nd.IPAddresses {
		if address.IP.String() == r.localNodeIP {
			id = identity.ReservedIdentityHost
		}
	}
	for _, address := range nd.IPAddresses {
		r.c.Upsert(address.ToString(), nil, 0, nil, ipc.Identity{ID: id, Source: source.Kubernetes})
		r.l.Debug("Added IP to ipcache", zap.String("IP", address.ToString()))
	}

	r.l.Info("Added Node", zap.String("Node", node.Name))
}

func (r *NodeReconciler) deleteNode(node *corev1.Node) {
	r.m.Lock()
	defer r.m.Unlock()
	nd, ok := r.nodes[node.Name]
	if !ok {
		r.l.Warn("Node not found", zap.String("Node", node.Name))
		return
	}
	delete(r.nodes, node.Name)

	for _, handler := range r.handlers {
		err := handler.NodeDelete(nd)
		if err != nil {
			r.l.Error("Failed to delete Node from datapath handler", zap.Error(err), zap.String("handler", handler.Name()), zap.String("Node", node.Name))
		}
	}
	for _, address := range nd.IPAddresses {
		r.c.Delete(address.ToString(), source.Kubernetes)
		r.l.Debug("Deleted IP from ipcache", zap.String("IP", address.ToString()))
	}
	r.l.Debug("Deleted Node", zap.String("Node", node.Name))
}

func (r *NodeReconciler) Subscribe(nh datapath.NodeHandler) {
	r.l.Debug("Subscribing to datapath handler")
	r.m.RLock()
	defer r.m.RUnlock()

	r.handlers[nh.Name()] = nh
	for i := range r.nodes {
		node := r.nodes[i]
		if err := nh.NodeAdd(node); err != nil {
			r.l.Error("Failed to add Node to datapath handler", zap.Error(err), zap.String("Node", node.Name))
		}
	}
}

func (r *NodeReconciler) Unsubscribe(nh datapath.NodeHandler) {
	r.l.Debug("Unsubscribing from datapath handler")
	r.m.Lock()
	defer r.m.Unlock()
	delete(r.handlers, nh.Name())
}

// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list
func (r *NodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.l.Debug("Reconciling Node", zap.String("Node", req.NamespacedName.String()))

	node := &corev1.Node{}
	if err := apiretry.Do(
		func() error {
			return r.Client.Get(ctx, req.NamespacedName, node)
		},
	); err != nil {
		if errors.IsNotFound(err) {
			// Node deleted since reconcile request received.
			r.l.Debug("Node deleted since reconcile request received", zap.String("Node", req.NamespacedName.String()))
			node.Name = req.Name
			r.deleteNode(node)
			return ctrl.Result{}, nil
		}
		r.l.Error("Failed to fetch Node", zap.Error(err), zap.String("Node", req.NamespacedName.String()))
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !node.ObjectMeta.DeletionTimestamp.IsZero() {
		r.l.Info("Node is being deleted", zap.String("Node", req.Name))
		r.deleteNode(node)
		return ctrl.Result{}, nil
	}

	r.addNode(node)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.l.Debug("Setting up Node controller")
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Node{}).
		Complete(r)
}
