package namespacecontroller

import (
	"context"
	"time"

	api "github.com/microsoft/retina/crd/api/v1alpha1"
	"github.com/microsoft/retina/pkg/common"
	"github.com/microsoft/retina/pkg/controllers/cache"
	"github.com/microsoft/retina/pkg/log"
	mm "github.com/microsoft/retina/pkg/module/metrics"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	interval = 1 * time.Second
)

type NamespaceReconciler struct {
	client.Client

	cache         cache.CacheInterface
	metricsModule mm.IModule
	l             *log.ZapLogger
}

func New(client client.Client, cache cache.CacheInterface, metricsModule mm.IModule) *NamespaceReconciler {
	return &NamespaceReconciler{
		Client:        client,
		cache:         cache,
		metricsModule: metricsModule,
		l:             log.Logger().Named(string("NamespaceReconciler")),
	}
}

// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list
func (r *NamespaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Get Namespace and check if it has the annotation
	r.l.Info("Reconciling Namespace", zap.String("namespace", req.NamespacedName.String()))
	namespace := &corev1.Namespace{}
	// If namespace is deleted, we ignore the error and continue
	if err := r.Client.Get(ctx, req.NamespacedName, namespace); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, err
		}
		namespace.Name = req.Name
		namespace.Namespace = req.Namespace
	}
	// if the namespace has an annotation and was not deleted
	if namespace.GetAnnotations()[common.RetinaPodAnnotation] == common.RetinaPodAnnotationValue && namespace.DeletionTimestamp.IsZero() {
		r.l.Info("Namespace has annotation", zap.String("namespace", namespace.Name))
		r.cache.AddAnnotatedNamespace(namespace.Name)
	} else {
		// namespace updated with annotation removed or the namespace was deleted
		r.l.Info("Namespace does not have annotation", zap.String("namespace", namespace.Name), zap.Any("annotations", namespace.GetAnnotations()))
		r.cache.DeleteAnnotatedNamespace(namespace.Name)
	}
	return ctrl.Result{}, nil
}

// Reconciles the metrics module with included namespaces from annotation on namespaces
func (r *NamespaceReconciler) Start(ctx context.Context) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			ns := r.cache.GetAnnotatedNamespaces()
			r.l.Debug("Reconciling metrics module", zap.Any("namespaces", ns))
			spec := (&api.MetricsSpec{}).
				WithIncludedNamespaces(ns).
				WithMetricsContextOptions(mm.DefaultMetrics(), mm.DefaultCtxOptions(), mm.DefaultCtxOptions())
			// MetricsModule will check the diff between namespaces and spec when reconciling.
			r.metricsModule.Reconcile(spec)
		case <-ctx.Done():
			return
		}
	}
}

func getPredicateFuncs() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			_, hasAnnotation := e.Object.GetAnnotations()[common.RetinaPodAnnotation]
			return hasAnnotation
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			_, annotOld := e.ObjectNew.GetAnnotations()[common.RetinaPodAnnotation]
			_, annotNew := e.ObjectOld.GetAnnotations()[common.RetinaPodAnnotation]
			return annotOld || annotNew
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			_, hasAnnotation := e.Object.GetAnnotations()[common.RetinaPodAnnotation]
			return hasAnnotation
		},
	}
}

func (r *NamespaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.l.Info("Setting up Namespace controller")
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Namespace{}).
		WithEventFilter(getPredicateFuncs()).
		Complete(r)
}
