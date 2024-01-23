/*
Copyright 2023.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package metricsconfigurationcontroller

import (
	"context"
	"sync"

	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	retinav1alpha1 "github.com/microsoft/retina/crd/api/v1alpha1"
	"github.com/microsoft/retina/crd/api/v1alpha1/validations"
	"github.com/microsoft/retina/pkg/log"
	mm "github.com/microsoft/retina/pkg/module/metrics"
)

// MetricsConfigurationReconciler reconciles a MetricsConfiguration object
type MetricsConfigurationReconciler struct {
	*sync.Mutex
	client.Client
	Scheme        *runtime.Scheme
	mcCache       map[string]*retinav1alpha1.MetricsConfiguration
	metricsModule *mm.Module
	l             *log.ZapLogger
}

func New(client client.Client, scheme *runtime.Scheme, metricsModule *mm.Module) *MetricsConfigurationReconciler {
	return &MetricsConfigurationReconciler{
		Mutex:         &sync.Mutex{},
		l:             log.Logger().Named(string("metricsconfiguration-controller")),
		Client:        client,
		Scheme:        scheme,
		mcCache:       make(map[string]*retinav1alpha1.MetricsConfiguration),
		metricsModule: metricsModule,
	}
}

//+kubebuilder:rbac:groups=operator.retina.io,resources=metricsconfiguration,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=operator.retina.io,resources=metricsconfiguration/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=operator.retina.io,resources=metricsconfiguration/finalizers,verbs=update

func (r *MetricsConfigurationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	mcc := &retinav1alpha1.MetricsConfiguration{}
	if err := r.Client.Get(ctx, req.NamespacedName, mcc); err != nil {
		if apierrors.IsNotFound(err) {
			// Object not found, it has probably been deleted
			r.l.Info("deleted", zap.String("name", req.NamespacedName.String()))
			r.Lock()
			if _, ok := r.mcCache[req.NamespacedName.String()]; ok {
				delete(r.mcCache, req.NamespacedName.String())
				r.l.Info("deleted from cache", zap.String("name", req.NamespacedName.String()))
			}
			r.Unlock()
			return ctrl.Result{}, nil
		} else {
			r.l.Info("error getting metricsconfiguration", zap.String("name", req.NamespacedName.String()))
			return ctrl.Result{}, err
		}
	}

	r.l.Info("reconciled", zap.String("name", req.NamespacedName.String()))
	r.Lock()
	defer r.Unlock()
	if len(r.mcCache) == 0 || r.mcCache[req.NamespacedName.String()] != nil {
		if mcc.Status.State != retinav1alpha1.StateAccepted {
			r.l.Info("ignoring this CRD as it is not configured and accepted by operator")
		} else {
			r.l.Info("adding to cache", zap.String("name", req.NamespacedName.String()))
			currentMcc := r.mcCache[req.NamespacedName.String()]

			if validations.CompareMetricsConfig(currentMcc, mcc) {
				r.l.Info("no change in metrics configuration, skipping reconcile", zap.String("name", req.NamespacedName.String()))
				return ctrl.Result{}, nil
			}

			err := r.metricsModule.Reconcile(&mcc.Spec)
			if err != nil {
				r.l.Info("error reconciling metrics configurations", zap.String("name", req.NamespacedName.String()))
			}
			r.mcCache[req.NamespacedName.String()] = mcc
		}
	} else {
		r.l.Error("Metrics Configuration is already configured on this cluster, cannot reconcile this new config.", zap.String("name", req.NamespacedName.String()))
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MetricsConfigurationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&retinav1alpha1.MetricsConfiguration{}).
		Complete(r)
}
