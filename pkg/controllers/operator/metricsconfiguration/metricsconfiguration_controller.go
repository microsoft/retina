package metricsconfigurationcontroller

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	retinav1alpha1 "github.com/microsoft/retina/crd/api/v1alpha1"
	validate "github.com/microsoft/retina/crd/api/v1alpha1/validations"
	"github.com/microsoft/retina/pkg/log"
)

// MetricsConfigurationReconciler reconciles a MetricsConfiguration object
type MetricsConfigurationReconciler struct {
	*sync.Mutex
	client.Client
	Scheme  *runtime.Scheme
	mcCache map[string]*retinav1alpha1.MetricsConfiguration
	l       *log.ZapLogger
}

func init() {
	log.SetupZapLogger(&log.LogOpts{
		File: false,
	})
}

func New(client client.Client, scheme *runtime.Scheme) *MetricsConfigurationReconciler {
	return &MetricsConfigurationReconciler{
		Mutex:   &sync.Mutex{},
		l:       log.Logger().Named(string("metricsconfiguration-controller")),
		Client:  client,
		Scheme:  scheme,
		mcCache: make(map[string]*retinav1alpha1.MetricsConfiguration),
	}
}

//+kubebuilder:rbac:groups=operator.retina.sh,resources=metricsconfigurations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=operator.retina.sh,resources=metricsconfigurations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=operator.retina.sh,resources=metricsconfigurations/finalizers,verbs=update

func (r *MetricsConfigurationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	mcc := &retinav1alpha1.MetricsConfiguration{}
	if err := r.Client.Get(ctx, req.NamespacedName, mcc); err != nil {
		if apierrors.IsNotFound(err) {
			// Object not found, it has probably been deleted
			r.l.Info("deleted", zap.String("name", req.NamespacedName.String()))
			r.Lock()
			_, ok := r.mcCache[req.NamespacedName.String()]
			if ok {
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
		err := validate.MetricsCRD(mcc)
		if err != nil {
			r.l.Error("Error validating metrics configuration", zap.Error(err))

			mcc.Status = retinav1alpha1.MetricsStatus{
				State:  retinav1alpha1.StateErrored,
				Reason: fmt.Sprintf("Validation of CRD failed with: %s", err.Error()),
			}
		} else {
			r.l.Info("metrics configuration is valid", zap.String("crd Name", mcc.Name))
			mcc.Status = retinav1alpha1.MetricsStatus{
				State:  retinav1alpha1.StateAccepted,
				Reason: "CRD is Accepted",
			}
			r.mcCache[req.NamespacedName.String()] = mcc
		}
	} else {
		r.l.Info("Metrics Conguration is already configured on this cluster, cannot reconcile this new config.")

		mcc.Status = retinav1alpha1.MetricsStatus{
			State:  retinav1alpha1.StateErrored,
			Reason: "Metrics Configuration is already configured on this cluster, cannot reconcile this new config.",
		}
	}

	if err := r.Client.Status().Update(ctx, mcc); err != nil {
		r.l.Error("Error updating metrics configuration", zap.Error(err))
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MetricsConfigurationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&retinav1alpha1.MetricsConfiguration{}).
		Complete(r)
}
