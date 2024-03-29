package tracesconfigurationcontroller

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	retinav1alpha1 "github.com/microsoft/retina/crd/api/v1alpha1"
)

// TracesConfigurationReconciler reconciles a TracesConfiguration object
type TracesConfigurationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=operator.retina.sh,resources=tracesconfiguration,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=operator.retina.sh,resources=tracesconfiguration/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=operator.retina.sh,resources=tracesconfiguration/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *TracesConfigurationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// TODO: implement the reconcile logic

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TracesConfigurationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&retinav1alpha1.TracesConfiguration{}).
		Complete(r)
}
