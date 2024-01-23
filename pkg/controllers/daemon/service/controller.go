// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package service

import (
	"context"
	"net"

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

// ServiceReconciler reconciles a Service object
type ServiceReconciler struct {
	client.Client

	cache *cache.Cache
	l     *log.ZapLogger
}

func New(client client.Client, cache *cache.Cache) *ServiceReconciler {
	return &ServiceReconciler{
		Client: client,
		cache:  cache,
		l:      log.Logger().Named(string("ServiceReconciler")),
	}
}

// +kubebuilder:rbac:groups="",resources=services,verbs=get;list
func (r *ServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// startTime := time.Now()
	r.l.Debug("Reconciling Service", zap.String("Service", req.NamespacedName.String()))
	/*
		// commenting out the latency calculation for now
		// we will need to create these as metrics
		defer func() {
			latency := time.Since(startTime).String()
			r.l.Info("Reconciliation ends", zap.String("Service", req.NamespacedName.String()), zap.String("latency", latency))
		}()
	*/

	service := &corev1.Service{}
	if err := apiretry.Do(
		func() error {
			return r.Client.Get(ctx, req.NamespacedName, service)
		},
	); err != nil {
		if errors.IsNotFound(err) {
			// RetinaService deleted since reconcile request received.
			r.l.Debug("Service is not found", zap.String("Service", req.NamespacedName.String()))
			// It's safe to use req.NamespacedName as the Service and RetinaService share the same namespace and name.
			if err := r.cache.DeleteRetinaSvc(req.NamespacedName.String()); err != nil {
				r.l.Warn("Failed to delete RetinaService in Cache from Service", zap.Error(err), zap.String("RetinaService", req.NamespacedName.String()))
			}
			return ctrl.Result{}, nil
		}
		r.l.Error("Failed to fetch Service", zap.Error(err), zap.String("Service", req.NamespacedName.String()))
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !service.ObjectMeta.DeletionTimestamp.IsZero() {
		r.l.Info("Service is being deleted", zap.String("Service", req.Name))
		if err := r.cache.DeleteRetinaSvc(req.NamespacedName.String()); err != nil {
			r.l.Warn("Failed to delete RetinaService in Cache from Service", zap.Error(err), zap.String("Service", req.NamespacedName.String()))
		}
		return ctrl.Result{}, nil
	}
	ips := retinaCommon.IPAddresses{}
	ips.IPv4 = net.ParseIP(service.Spec.ClusterIP)
	net.ParseIP(service.Spec.ClusterIP)
	var lbIP net.IP
	if service.Status.LoadBalancer.Ingress != nil && len(service.Status.LoadBalancer.Ingress) > 0 {
		lbIP = net.ParseIP(service.Status.LoadBalancer.Ingress[0].IP)
	}
	retinaSvcCommon := retinaCommon.NewRetinaSvc(service.Name, service.Namespace, &ips, lbIP, service.Spec.Selector)
	if err := r.cache.UpdateRetinaSvc(retinaSvcCommon); err != nil {
		r.l.Error("Failed to update RetinaService in Cache", zap.Error(err), zap.String("Service", req.NamespacedName.String()))
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.l.Info("Setting up Service controller")
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Service{}).
		Complete(r)
}
