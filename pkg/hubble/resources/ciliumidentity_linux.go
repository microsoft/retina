package resources

import (
	"context"
	"sync"

	cid "github.com/cilium/cilium/pkg/identity"
	v2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	"github.com/cilium/hive/cell"
	"github.com/microsoft/retina/pkg/hubble/common"
	"github.com/microsoft/retina/pkg/log"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

var _ common.LabelCache = (*CiliumIdentityReconciler)(nil)

const (
	CiliumIdentityReconcilerName = "cilium-identity-reconciler"
)

// CiliumIdentityReconciler implements a Kubernetes CiliumIdentity reconciler using controller-runtime
type CiliumIdentityReconciler struct {
	client.Client

	// Internal cache for identity management
	mu                 sync.RWMutex
	labelsByIdentityID map[cid.NumericIdentity][]string

	logger *log.ZapLogger
}

type CiliumIdentityReconcilerOut struct {
	cell.Out

	common.LabelCache
	*CiliumIdentityReconciler
}

// NewCiliumIdentityReconciler creates a new CiliumIdentityReconciler instance
func NewCiliumIdentityReconciler(cli client.Client) CiliumIdentityReconcilerOut {
	reconciler := &CiliumIdentityReconciler{
		Client:             cli,
		labelsByIdentityID: make(map[cid.NumericIdentity][]string),
		logger:             log.Logger().Named(CiliumIdentityReconcilerName),
	}
	return CiliumIdentityReconcilerOut{
		LabelCache:               reconciler,
		CiliumIdentityReconciler: reconciler,
	}
}

// +kubebuilder:rbac:groups=cilium.io,resources=ciliumidentities,verbs=get;list;watch
//
// Reconcile implements the main reconciliation logic for CiliumIdentity
func (r *CiliumIdentityReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.logger.Debug("Reconciling CiliumIdentity", zap.String("identity", req.String()))

	identity := &v2.CiliumIdentity{}
	err := r.Get(ctx, req.NamespacedName, identity)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			// Identity was deleted
			r.logger.Debug("CiliumIdentity not found, removing from cache", zap.String("identity", req.String()))
			r.removeIdentityFromCache(req.NamespacedName)
			return ctrl.Result{}, nil
		}
		r.logger.Error("Failed to get CiliumIdentity", zap.Error(err), zap.String("identity", req.String()))
		return ctrl.Result{}, errors.Wrap(err, "failed to get CiliumIdentity")
	}

	// Handle identity deletion
	if !identity.DeletionTimestamp.IsZero() {
		r.logger.Info("CiliumIdentity is being deleted", zap.String("identity", req.String()))
		r.removeIdentityFromCache(req.NamespacedName)
		return ctrl.Result{}, nil
	}

	// Update identity cache
	return r.updateIdentityCache(identity)
}

func (r *CiliumIdentityReconciler) GetLabelsFromSecurityIdentity(id cid.NumericIdentity) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	// Retrieve labels from the cache
	labels, exists := r.labelsByIdentityID[id]
	if !exists {
		r.logger.Debug("Identity not found in cache", zap.Uint32("identity", id.Uint32()))
		return nil
	}
	r.logger.Debug("Retrieved labels from cache", zap.Uint32("identity", id.Uint32()), zap.Strings("labels", labels))
	return labels
}

func (r *CiliumIdentityReconciler) updateIdentityCache(identity *v2.CiliumIdentity) (ctrl.Result, error) {
	id, err := cid.ParseNumericIdentity(identity.Name)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "invalid identity name %s", identity.Name)
	}
	// Parse the security labels
	secLabels := make([]string, 0, len(identity.SecurityLabels))
	for k, v := range identity.SecurityLabels {
		secLabels = append(secLabels, k+"="+v)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	// Update the cache with the new labels
	r.labelsByIdentityID[id] = secLabels

	return ctrl.Result{}, nil
}

func (r *CiliumIdentityReconciler) removeIdentityFromCache(name client.ObjectKey) {
	r.mu.Lock()
	defer r.mu.Unlock()

	id, err := cid.ParseNumericIdentity(name.Name)
	if err != nil {
		r.logger.Error("Failed to parse identity name", zap.Error(err), zap.String("identity", name.Name))
		return
	}

	if _, exists := r.labelsByIdentityID[id]; exists {
		delete(r.labelsByIdentityID, id)
		r.logger.Debug("Removed identity from cache", zap.String("identity", name.Name))
	} else {
		r.logger.Debug("Identity not found in cache", zap.String("identity", name.Name))
	}
}

// identityChangedPredicate returns a predicate that triggers reconciliation on meaningful changes
func identityChangedPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(event.CreateEvent) bool {
			// Always reconcile on creation
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldIdentity, ok := e.ObjectOld.(*v2.CiliumIdentity)
			if !ok {
				return false
			}
			newIdentity, ok := e.ObjectNew.(*v2.CiliumIdentity)
			if !ok {
				return false
			}

			// Reconcile if security labels changed
			return !mapsEqual(oldIdentity.SecurityLabels, newIdentity.SecurityLabels)
		},
		DeleteFunc: func(event.DeleteEvent) bool {
			// Always reconcile on deletion
			return true
		},
		GenericFunc: func(event.GenericEvent) bool {
			return false
		},
	}
}

// mapsEqual compares two string maps for equality
func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || bv != v {
			return false
		}
	}
	return true
}

// SetupWithManager sets up the controller with the Manager
func (r *CiliumIdentityReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Setup CiliumIdentity controller with change predicate
	err := ctrl.NewControllerManagedBy(mgr).
		For(&v2.CiliumIdentity{}).
		WithEventFilter(identityChangedPredicate()).
		Complete(r)
	if err != nil {
		return errors.Wrap(err, "failed to set up CiliumIdentity reconciler")
	}

	r.logger.Info("CiliumIdentity reconciler setup completed")
	return nil
}
