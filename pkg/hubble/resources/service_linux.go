package resources

import (
	"context"
	"net/netip"
	"sync"

	"github.com/cilium/cilium/api/v1/flow"
	"github.com/cilium/cilium/pkg/k8s/resource"
	slim_corev1 "github.com/cilium/cilium/pkg/k8s/slim/k8s/api/core/v1"
	"github.com/cilium/hive/cell"
	"github.com/microsoft/retina/pkg/hubble/common"
	"github.com/microsoft/retina/pkg/log"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/types"
)

var _ common.SvcDecoder = (*ServiceHandler)(nil)

const (
	ServiceHandlerName = "k8s-service-handler"
)

// ServiceHandler watches Kubernetes services using resource.Resource
type ServiceHandler struct {
	services resource.Resource[*slim_corev1.Service]

	// Internal cache for services and their IPs
	mu            sync.RWMutex
	ipToService   map[string]*flow.Service          // Maps IP to *flow.Service
	serviceToIPs  map[types.NamespacedName][]string // Maps service key to its IPs
	ctxCancelFunc context.CancelFunc

	logger *log.ZapLogger
}

type ServiceHandlerParams struct {
	cell.In

	Lifecycle cell.Lifecycle
	Services  resource.Resource[*slim_corev1.Service]
}

type ServiceHandlerOut struct {
	cell.Out

	common.SvcDecoder
	*ServiceHandler
}

// NewServiceHandler creates a new ServiceHandler instance
func NewServiceHandler(params ServiceHandlerParams) ServiceHandlerOut {
	handler := &ServiceHandler{
		services:     params.Services,
		ipToService:  make(map[string]*flow.Service),
		serviceToIPs: make(map[types.NamespacedName][]string),
		logger:       log.Logger().Named(ServiceHandlerName),
	}

	params.Lifecycle.Append(handler)

	return ServiceHandlerOut{
		SvcDecoder:     handler,
		ServiceHandler: handler,
	}
}

func (h *ServiceHandler) Start(cell.HookContext) error {
	ctx, cancel := context.WithCancel(context.Background())
	h.ctxCancelFunc = cancel
	go h.run(ctx)
	h.logger.Info("Service handler started")
	return nil
}

func (h *ServiceHandler) Stop(cell.HookContext) error {
	if h.ctxCancelFunc != nil {
		h.ctxCancelFunc()
	}
	h.logger.Info("Service handler stopped")
	return nil
}

func (h *ServiceHandler) run(ctx context.Context) {
	h.logger.Info("Starting service event handler")

	serviceEvents := h.services.Events(ctx)

	for {
		select {
		case ev, ok := <-serviceEvents:
			if !ok {
				h.logger.Info("Service events channel closed")
				return
			}

			h.handleEvent(ev)
			ev.Done(nil)

		case <-ctx.Done():
			h.logger.Info("Service event handler stopped")
			return
		}
	}
}

func (h *ServiceHandler) handleEvent(ev resource.Event[*slim_corev1.Service]) {
	switch ev.Kind {
	case resource.Sync:
		// Ignore sync events
		h.logger.Debug("Service sync event received", zap.String("key", ev.Key.String()))
	case resource.Upsert:
		if ev.Object != nil {
			h.logger.Debug("Service upsert event", zap.String("key", ev.Key.String()))
			h.updateServiceCache(ev.Object)
		}
	case resource.Delete:
		h.logger.Debug("Service delete event", zap.String("key", ev.Key.String()))
		h.removeServiceFromCache(ev.Key.Namespace, ev.Key.Name)
	}
}

// Decode returns the service associated with the given IP address, or nil if not found.
func (h *ServiceHandler) Decode(ip netip.Addr) *flow.Service {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.ipToService[ip.String()]
}

// updateServiceCache updates the internal service cache
func (h *ServiceHandler) updateServiceCache(service *slim_corev1.Service) {
	h.mu.Lock()
	defer h.mu.Unlock()

	key := types.NamespacedName{
		Namespace: service.Namespace,
		Name:      service.Name,
	}

	// Remove old IP mappings first
	if oldIPs, exists := h.serviceToIPs[key]; exists {
		for _, oldIP := range oldIPs {
			delete(h.ipToService, oldIP)
		}
	}

	// Update IP mappings with new values
	h.addServiceIPMappings(service)

	h.logger.Debug("Updated service cache",
		zap.String("service", key.String()),
		zap.Strings("clusterIPs", service.Spec.ClusterIPs))
}

// removeServiceFromCache removes a service from the cache
func (h *ServiceHandler) removeServiceFromCache(namespace, name string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	serviceKey := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}

	if ips, exists := h.serviceToIPs[serviceKey]; exists {
		for _, ip := range ips {
			delete(h.ipToService, ip)
		}
		delete(h.serviceToIPs, serviceKey)
	}
}

// addServiceIPMappings adds IP to service mappings
func (h *ServiceHandler) addServiceIPMappings(service *slim_corev1.Service) {
	key := types.NamespacedName{
		Namespace: service.Namespace,
		Name:      service.Name,
	}

	flowService := &flow.Service{
		Name:      service.Name,
		Namespace: service.Namespace,
	}

	h.serviceToIPs[key] = []string{}

	// Add ClusterIP mapping
	for _, clusterIP := range service.Spec.ClusterIPs {
		if clusterIP != "" && clusterIP != "None" {
			h.ipToService[clusterIP] = flowService // Use the same cached object
			h.serviceToIPs[key] = append(h.serviceToIPs[key], clusterIP)
		}
	}
}
