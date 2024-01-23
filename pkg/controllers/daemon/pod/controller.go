// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package pod

import (
	"context"

	"github.com/microsoft/retina/pkg/common/apiretry"
	"github.com/microsoft/retina/pkg/log"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	errors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	retinaCommon "github.com/microsoft/retina/pkg/common"
	"github.com/microsoft/retina/pkg/controllers/cache"
)

// PodReconciler reconciles a Pod object
type PodReconciler struct {
	client.Client

	cache *cache.Cache
	l     *log.ZapLogger
}

func New(client client.Client, cache *cache.Cache) *PodReconciler {
	return &PodReconciler{
		Client: client,
		cache:  cache,
		l:      log.Logger().Named(string("PodReconciler")),
	}
}

// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list
func (r *PodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.l.Info("Reconciling Pod", zap.String("Pod", req.NamespacedName.String()))

	pod := &corev1.Pod{}
	if err := apiretry.Do(
		func() error {
			return r.Client.Get(ctx, req.NamespacedName, pod)
		},
	); err != nil {
		if errors.IsNotFound(err) {
			// RetinaEndpoint deleted since reconcile request received.
			r.l.Debug("Pod is not found", zap.String("Pod", req.NamespacedName.String()))
			// It's safe to use req.NamespacedName as the Pod and RetinaEndpoint share the same namespace and name.
			if err := r.cache.DeleteRetinaEndpoint(req.NamespacedName.String()); err != nil {
				r.l.Warn("Failed to delete RetinaEndpoint in Cache from Pod", zap.Error(err), zap.String("RetinaEndpoint", req.NamespacedName.String()))
			}
			return ctrl.Result{}, nil
		}
		r.l.Error("Failed to fetch Pod", zap.Error(err), zap.String("Pod", req.NamespacedName.String()))
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if pod.Spec.HostNetwork {
		r.l.Debug("Ignoring host network pod", zap.String("Pod", req.NamespacedName.String()))
		return ctrl.Result{}, nil
	}

	if !pod.ObjectMeta.DeletionTimestamp.IsZero() {
		r.l.Info("Pod is being deleted", zap.String("Pod", req.Name))
		if err := r.cache.DeleteRetinaEndpoint(req.NamespacedName.String()); err != nil {
			r.l.Warn("Failed to delete RetinaEndpoint in Cache from Pod", zap.Error(err), zap.String("Pod", req.NamespacedName.String()))
		}
		return ctrl.Result{}, nil
	}

	if pod.Status.PodIP == "" {
		r.l.Debug("Pod has no IP address", zap.String("Pod", req.NamespacedName.String()))
		return ctrl.Result{}, nil
	}

	retinaEndpointCommon := retinaCommon.RetinaEndpointCommonFromPod(pod)
	if err := r.cache.UpdateRetinaEndpoint(retinaEndpointCommon); err != nil {
		r.l.Error("Failed to update RetinaEndpoint in Cache", zap.Error(err), zap.String("Pod", req.NamespacedName.String()))
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.l.Info("Setting up Pod controller")
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		Complete(r)
}
