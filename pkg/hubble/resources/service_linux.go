package resources

import (
	"context"
	"fmt"
	"net/netip"
	"reflect"
	"sync"

	"github.com/cilium/cilium/api/v1/flow"
	"github.com/microsoft/retina/pkg/hubble/common"
	"github.com/microsoft/retina/pkg/log"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

var _ common.SvcDecoder = (*ServiceReconciler)(nil)

const (
	ServiceReconcilerName = "service-reconciler"
)

// ServiceReconciler implements a Kubernetes service reconciler using controller-runtime
type ServiceReconciler struct {
	client.Client

	// Internal cache for services and their IPs
	mu           sync.RWMutex
	ipToService  map[string]*flow.Service          // Maps IP to *flow.Service
	serviceToIPs map[types.NamespacedName][]string // Maps service key to its IPs

	logger *log.ZapLogger
}

// NewServiceReconciler creates a new ServiceReconciler instance
func NewServiceReconciler(cli client.Client) *ServiceReconciler {
	return &ServiceReconciler{
		Client:       cli,
		ipToService:  make(map[string]*flow.Service),
		serviceToIPs: make(map[types.NamespacedName][]string),
		logger:       log.Logger().Named(ServiceReconcilerName),
	}
}

// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch
//
// Reconcile implements the main reconciliation logic for services
func (r *ServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.logger.Debug("Reconciling Service", zap.String("service", req.String()))

	service := &corev1.Service{}
	err := r.Get(ctx, req.NamespacedName, service)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			// Service was deleted
			r.logger.Debug("Service not found, removing from cache", zap.String("service", req.String()))
			r.removeServiceFromCache(req.NamespacedName)
			return ctrl.Result{}, nil
		}
		r.logger.Error("Failed to get Service", zap.Error(err), zap.String("service", req.String()))
		return ctrl.Result{}, errors.Wrap(err, "failed to get Service")
	}

	// Handle service deletion
	if !service.DeletionTimestamp.IsZero() {
		r.logger.Info("Service is being deleted", zap.String("service", req.String()))
		r.removeServiceFromCache(req.NamespacedName)
		return ctrl.Result{}, nil
	}

	// Update service cache
	r.updateServiceCache(service)

	return ctrl.Result{}, nil
}

func (r *ServiceReconciler) Decode(ip netip.Addr) *flow.Service {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.ipToService[ip.String()]
}

// updateServiceCache updates the internal service cache
func (r *ServiceReconciler) updateServiceCache(service *corev1.Service) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := types.NamespacedName{
		Namespace: service.Namespace,
		Name:      service.Name,
	}

	// Remove old IP mappings first
	if oldIPs, exists := r.serviceToIPs[key]; exists {
		for _, oldIP := range oldIPs {
			delete(r.ipToService, oldIP)
		}
	}

	// Update IP mappings with new values
	r.addServiceIPMappings(service)

	r.logger.Debug("Updated service cache",
		zap.String("service", key.String()),
		zap.Strings("clusterIPs", service.Spec.ClusterIPs))
}

// removeServiceFromCache removes a service from the cache
func (r *ServiceReconciler) removeServiceFromCache(serviceKey types.NamespacedName) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if ips, exists := r.serviceToIPs[serviceKey]; exists {
		for _, ip := range ips {
			delete(r.ipToService, ip)
		}
		delete(r.serviceToIPs, serviceKey)
	}
}

// addServiceIPMappings adds IP to service mappings
func (r *ServiceReconciler) addServiceIPMappings(service *corev1.Service) {
	key := types.NamespacedName{
		Namespace: service.Namespace,
		Name:      service.Name,
	}

	flowService := &flow.Service{
		Name:      service.Name,
		Namespace: service.Namespace,
	}

	r.serviceToIPs[key] = []string{}

	// Add ClusterIP mapping
	for _, clusterIP := range service.Spec.ClusterIPs {
		if clusterIP != "" && clusterIP != "None" {
			r.ipToService[clusterIP] = flowService // Use the same cached object
			r.serviceToIPs[key] = append(r.serviceToIPs[key], clusterIP)
		}
	}
}

// clusterIPsChangedPredicate returns a predicate that only allows reconciliation when ClusterIPs change
func clusterIPsChangedPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			service, ok := e.Object.(*corev1.Service)
			if !ok {
				return false
			}
			// Only reconcile services with ClusterIPs (exclude headless services)
			return hasValidClusterIPs(service)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldService, ok := e.ObjectOld.(*corev1.Service)
			if !ok {
				return false
			}
			newService, ok := e.ObjectNew.(*corev1.Service)
			if !ok {
				return false
			}

			// Only reconcile if either service has ClusterIPs and ClusterIPs have changed
			hasOldClusterIPs := hasValidClusterIPs(oldService)
			hasNewClusterIPs := hasValidClusterIPs(newService)

			// Reconcile if:
			// 1. Service gained ClusterIPs (headless -> non-headless)
			// 2. Service lost ClusterIPs (non-headless -> headless)
			// 3. Service has ClusterIPs and they changed
			if hasOldClusterIPs != hasNewClusterIPs {
				return true
			}

			// If both have ClusterIPs, check if they changed
			if hasNewClusterIPs {
				return !reflect.DeepEqual(oldService.Spec.ClusterIPs, newService.Spec.ClusterIPs)
			}

			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			service, ok := e.Object.(*corev1.Service)
			if !ok {
				return false
			}
			// Only reconcile deletion of services that had ClusterIPs
			return hasValidClusterIPs(service)
		},
		GenericFunc: func(event.GenericEvent) bool {
			return false
		},
	}
}

// hasValidClusterIPs checks if a service has valid ClusterIPs (not headless)
func hasValidClusterIPs(service *corev1.Service) bool {
	if len(service.Spec.ClusterIPs) == 0 {
		return false
	}

	for _, clusterIP := range service.Spec.ClusterIPs {
		if clusterIP != "" && clusterIP != "None" {
			return true
		}
	}

	return false
}

// SetupWithManager sets up the controller with the Manager
func (r *ServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Setup service controller with ClusterIP change predicate
	err := ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Service{}).
		WithEventFilter(clusterIPsChangedPredicate()).
		Complete(r)
	if err != nil {
		return fmt.Errorf("failed to setup service controller: %w", err)
	}

	r.logger.Info("Service reconciler setup completed")
	return nil
}
