// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package nodereconciler

import (
	"context"
	"fmt"
	"net"
	"reflect"
	"sync"

	"github.com/microsoft/retina/pkg/common/apiretry"
	"github.com/microsoft/retina/pkg/log"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	errors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	cmtypes "github.com/cilium/cilium/pkg/clustermesh/types"
	datapath "github.com/cilium/cilium/pkg/datapath/types"
	"github.com/cilium/cilium/pkg/identity"
	ipc "github.com/cilium/cilium/pkg/ipcache"
	"github.com/cilium/cilium/pkg/node/addressing"
	"github.com/cilium/cilium/pkg/node/types"
	"github.com/cilium/cilium/pkg/source"
	"github.com/cilium/cilium/pkg/time"
)

// NodeReconciler reconciles a Node object.
// This is pretty basic for now, need fine tuning, scale test, etc.
type NodeReconciler struct {
	client.Client

	clusterName string

	l           *log.ZapLogger
	handlers    map[string]datapath.NodeHandler
	nodes       map[string]types.Node
	c           *ipc.IPCache
	localNodeIP string
	m           sync.RWMutex
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
	if _, ok := r.nodes[node.Name]; ok {
		r.l.Debug("Node already exists", zap.String("Node", node.Name))
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
		_, err := r.c.Upsert(address.ToString(), nil, 0, nil, ipc.Identity{ID: id, Source: source.Kubernetes}) //nolint:staticcheck // TODO(timraymond): no clear upgrade path
		if err != nil {
			r.l.Debug("failed to add IP to ipcache", zap.Error(err))
		}
		r.l.Debug("Added IP to ipcache", zap.String("IP", address.ToString()))
	}

	r.l.Info("Added Node", zap.String("name", nd.Name))
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
		//nolint:staticcheck // TODO(timraymond): unhelpful deprecation notice: migration path unclear
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
			err := r.Client.Get(ctx, req.NamespacedName, node)
			if err != nil {
				return fmt.Errorf("getting node: %w", err)
			}
			return nil
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
		return ctrl.Result{}, fmt.Errorf("retrieving node info: %w", err)
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

	// Create a predicate to filter node events
	nodePredicate := predicate.Funcs{
		CreateFunc: func(event.CreateEvent) bool {
			// Always reconcile on node creation
			return true
		},
		DeleteFunc: func(event.DeleteEvent) bool {
			// Always reconcile on node deletion
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldNode, ok := e.ObjectOld.(*corev1.Node)
			if !ok {
				r.l.Error("Failed to convert old object to Node")
				return false
			}

			newNode, ok := e.ObjectNew.(*corev1.Node)
			if !ok {
				r.l.Error("Failed to convert new object to Node")
				return false
			}

			// Compare node IP addresses
			oldIPs := extractNodeIPs(oldNode)
			newIPs := extractNodeIPs(newNode)

			// Only reconcile if IPs changed
			return !reflect.DeepEqual(oldIPs, newIPs)
		},
		GenericFunc: func(event.GenericEvent) bool {
			return false
		},
	}

	err := ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Node{}).
		WithEventFilter(nodePredicate).
		Complete(r)
	if err != nil {
		return fmt.Errorf("setting up node controller: %w", err)
	}
	return nil
}

// The following methods are stubs for the NodeManager interface.
// It is done because the hubble requires NodeManager interface as a dependency.
// However, we don't need to implement all the methods.
// TODO: make Notifier interface as dependency for the hubble instead of NodeManager on upstream.
func (r *NodeReconciler) ClusterSizeDependantInterval(time.Duration) time.Duration {
	return time.Second * 5
}

func (r *NodeReconciler) Enqueue(*types.Node) {}

func (r *NodeReconciler) GetNodeIdentities() []types.Identity {
	return []types.Identity{}
}

func (r *NodeReconciler) GetNodes() map[types.Identity]types.Node {
	return map[types.Identity]types.Node{}
}

func (r *NodeReconciler) MeshNodeSync() {}

func (r *NodeReconciler) NodeDeleted(types.Node) {}

func (r *NodeReconciler) NodeSync() {}

func (r *NodeReconciler) NodeUpdated(types.Node) {}

func (r *NodeReconciler) StartNeighborRefresh(datapath.NodeNeighbors) {}

func (r *NodeReconciler) StartNodeNeighborLinkUpdater(datapath.NodeNeighbors) {}

func (r *NodeReconciler) SetPrefixClusterMutatorFn(func(*types.Node) []cmtypes.PrefixClusterOpts) {}

// extractNodeIPs extracts IP addresses from a node
func extractNodeIPs(node *corev1.Node) map[string]string {
	ips := make(map[string]string)
	for _, addr := range node.Status.Addresses {
		if addr.Type == corev1.NodeInternalIP || addr.Type == corev1.NodeExternalIP {
			ips[string(addr.Type)] = addr.Address
		}
	}
	return ips
}
