// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package retinaendpoint

import (
	"context"
	"time"

	"github.com/microsoft/retina/pkg/common/apiretry"
	"github.com/microsoft/retina/pkg/log"
	"go.uber.org/zap"
	errors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1 "github.com/microsoft/retina/crd/api/v1alpha1"
	retinaCommon "github.com/microsoft/retina/pkg/common"
	"github.com/microsoft/retina/pkg/controllers/cache"
)

// RetinaEndpointReconciler reconciles a RetinaEndpoint object
type RetinaEndpointReconciler struct {
	client.Client

	cache *cache.Cache
	l     *log.ZapLogger
}

func New(client client.Client, cache *cache.Cache) *RetinaEndpointReconciler {
	return &RetinaEndpointReconciler{
		Client: client,
		cache:  cache,
		l:      log.Logger().Named(string("RetinaEndpointReconciler")),
	}
}

// +kubebuilder:rbac:groups=retina.sh,resources=retinaendpoints,verbs=get;list;watch
func (r *RetinaEndpointReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	startTime := time.Now()
	r.l.Info("Reconciling RetinaEndpoint", zap.String("RetinaEndpoint", req.NamespacedName.String()))

	defer func() {
		latency := time.Since(startTime).String()
		r.l.Info("Reconciliation ends", zap.String("RetinaEndpoint", req.NamespacedName.String()), zap.String("latency", latency))
	}()

	retinaEndpoint := &operatorv1.RetinaEndpoint{}
	if err := apiretry.Do(
		func() error {
			return r.Get(ctx, req.NamespacedName, retinaEndpoint)
		},
	); err != nil {
		if errors.IsNotFound(err) {
			// RetinaEndpoint deleted since reconcile request received.
			r.l.Debug("RetinaEndpoint is not found", zap.String("RetinaEndpoint", req.NamespacedName.String()))
			// It's safe to use req.NamespacedName as the Pod and RetinaEndpoint share the same namespace and name.
			if err := r.cache.DeleteRetinaEndpoint(req.NamespacedName.String()); err != nil {
				r.l.Warn("Failed to delete RetinaEndpoint in Cache", zap.Error(err), zap.String("RetinaEndpoint", req.NamespacedName.String()))
			}
			return ctrl.Result{}, nil
		}
		r.l.Error("Failed to fetch RetinaEndpoint", zap.Error(err), zap.String("RetinaEndpoint", req.NamespacedName.String()))
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !retinaEndpoint.ObjectMeta.DeletionTimestamp.IsZero() {
		r.l.Info("RetinaEndpoint is being deleted", zap.String("RetinaEndpoint", req.NamespacedName.String()))
		if err := r.cache.DeleteRetinaEndpoint(req.NamespacedName.String()); err != nil {
			r.l.Warn("Failed to delete RetinaEndpoint in Cache", zap.Error(err), zap.String("RetinaEndpoint", req.NamespacedName.String()))
		}
		return ctrl.Result{}, nil
	}

	var zone string
	node := r.cache.GetNodeByIP(retinaEndpoint.Spec.NodeIP)
	if node != nil {
		zone = node.Zone()
	}
	retinaEndpointCommon := retinaCommon.RetinaEndpointCommonFromAPI(retinaEndpoint, zone)

	if err := r.cache.UpdateRetinaEndpoint(retinaEndpointCommon); err != nil {
		r.l.Error("Failed to update RetinaEndpoint in Cache", zap.Error(err), zap.String("RetinaEndpoint", req.NamespacedName.String()))
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RetinaEndpointReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.l.Info("Setting up RetinaEndpoint controller")
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1.RetinaEndpoint{}).
		Complete(r)
}
