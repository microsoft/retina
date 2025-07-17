package resources

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/cilium/cilium/api/v1/flow"
	"github.com/cilium/cilium/pkg/k8s/resource"
	slim_corev1 "github.com/cilium/cilium/pkg/k8s/slim/k8s/api/core/v1"
	slim_metav1 "github.com/cilium/cilium/pkg/k8s/slim/k8s/apis/meta/v1"
	"github.com/cilium/hive/cell"
	"github.com/microsoft/retina/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
)

func init() {
	_, _ = log.SetupZapLogger(log.GetDefaultLogOpts())
}

// mockServiceResource is a test implementation of resource.Resource[*slim_corev1.Service]
type mockServiceResource struct {
	eventChan chan resource.Event[*slim_corev1.Service]
}

func newMockServiceResource() *mockServiceResource {
	return &mockServiceResource{
		eventChan: make(chan resource.Event[*slim_corev1.Service], 100),
	}
}

func (m *mockServiceResource) Events(context.Context, ...resource.EventsOpt) <-chan resource.Event[*slim_corev1.Service] {
	return m.eventChan
}

func (m *mockServiceResource) Observe(context.Context, func(resource.Event[*slim_corev1.Service]), func(error)) {
}

func (m *mockServiceResource) Store(context.Context) (resource.Store[*slim_corev1.Service], error) {
	return nil, nil
}

func (m *mockServiceResource) sendEvent(kind resource.EventKind, svc *slim_corev1.Service) {
	key := resource.Key{}
	if svc != nil {
		key.Name = svc.Name
		key.Namespace = svc.Namespace
	}

	done := make(chan error, 1)
	m.eventChan <- resource.Event[*slim_corev1.Service]{
		Kind:   kind,
		Key:    key,
		Object: svc,
		Done:   func(err error) { done <- err },
	}
	// Wait for event to be processed
	<-done
}

func (m *mockServiceResource) close() {
	close(m.eventChan)
}

func TestNewServiceHandler(t *testing.T) {
	mockRes := newMockServiceResource()
	defer mockRes.close()

	lc := &cell.DefaultLifecycle{}
	params := ServiceHandlerParams{
		Lifecycle: lc,
		Services:  mockRes,
	}

	handlerOut := NewServiceHandler(params)

	assert.NotNil(t, handlerOut.ServiceHandler)
	assert.NotNil(t, handlerOut.SvcDecoder)
	assert.NotNil(t, handlerOut.ipToService)
	assert.NotNil(t, handlerOut.serviceToIPs)
	assert.NotNil(t, handlerOut.logger)
	assert.Equal(t, ServiceHandlerName, handlerOut.logger.Name())
}

func TestServiceHandler_EventHandling_ServiceCreated(t *testing.T) {
	mockRes := newMockServiceResource()
	defer mockRes.close()

	lc := &cell.DefaultLifecycle{}
	params := ServiceHandlerParams{
		Lifecycle: lc,
		Services:  mockRes,
	}

	handlerOut := NewServiceHandler(params)
	handler := handlerOut.ServiceHandler

	// Start the handler
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := handler.Start(ctx)
	require.NoError(t, err)

	// Give the handler time to start
	time.Sleep(10 * time.Millisecond)

	service := &slim_corev1.Service{
		ObjectMeta: slim_metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "default",
		},
		Spec: slim_corev1.ServiceSpec{
			ClusterIPs: []string{"10.96.0.1", "10.96.0.2"},
		},
	}

	// Send upsert event
	mockRes.sendEvent(resource.Upsert, service)

	// Give the handler time to process
	time.Sleep(10 * time.Millisecond)

	// Verify the service is cached
	handler.mu.RLock()
	defer handler.mu.RUnlock()

	expectedKey := types.NamespacedName{Name: "test-service", Namespace: "default"}
	assert.Contains(t, handler.ipToService, "10.96.0.1")
	assert.Contains(t, handler.ipToService, "10.96.0.2")

	flowService1 := handler.ipToService["10.96.0.1"]
	flowService2 := handler.ipToService["10.96.0.2"]
	assert.Equal(t, "test-service", flowService1.GetName())
	assert.Equal(t, "default", flowService1.GetNamespace())
	assert.Same(t, flowService1, flowService2)

	assert.Contains(t, handler.serviceToIPs, expectedKey)
	assert.ElementsMatch(t, []string{"10.96.0.1", "10.96.0.2"}, handler.serviceToIPs[expectedKey])
}

func TestServiceHandler_EventHandling_ServiceUpdated(t *testing.T) {
	mockRes := newMockServiceResource()
	defer mockRes.close()

	lc := &cell.DefaultLifecycle{}
	params := ServiceHandlerParams{
		Lifecycle: lc,
		Services:  mockRes,
	}

	handlerOut := NewServiceHandler(params)
	handler := handlerOut.ServiceHandler

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := handler.Start(ctx)
	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond)

	// Initial service
	service := &slim_corev1.Service{
		ObjectMeta: slim_metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "default",
		},
		Spec: slim_corev1.ServiceSpec{
			ClusterIPs: []string{"10.96.0.1", "10.96.0.2"},
		},
	}

	mockRes.sendEvent(resource.Upsert, service)
	time.Sleep(10 * time.Millisecond)

	// Update service with new IPs
	updatedService := &slim_corev1.Service{
		ObjectMeta: slim_metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "default",
		},
		Spec: slim_corev1.ServiceSpec{
			ClusterIPs: []string{"10.96.0.3", "10.96.0.4"},
		},
	}

	mockRes.sendEvent(resource.Upsert, updatedService)
	time.Sleep(10 * time.Millisecond)

	// Verify cache is updated
	handler.mu.RLock()
	defer handler.mu.RUnlock()

	expectedKey := types.NamespacedName{Name: "test-service", Namespace: "default"}

	// Old IPs should be gone
	assert.NotContains(t, handler.ipToService, "10.96.0.1")
	assert.NotContains(t, handler.ipToService, "10.96.0.2")

	// New IPs should be present
	assert.Contains(t, handler.ipToService, "10.96.0.3")
	assert.Contains(t, handler.ipToService, "10.96.0.4")

	flowService := handler.ipToService["10.96.0.3"]
	assert.Equal(t, "test-service", flowService.GetName())
	assert.Equal(t, "default", flowService.GetNamespace())

	assert.Contains(t, handler.serviceToIPs, expectedKey)
	assert.ElementsMatch(t, []string{"10.96.0.3", "10.96.0.4"}, handler.serviceToIPs[expectedKey])
}

func TestServiceHandler_EventHandling_ServiceDeleted(t *testing.T) {
	mockRes := newMockServiceResource()
	defer mockRes.close()

	lc := &cell.DefaultLifecycle{}
	params := ServiceHandlerParams{
		Lifecycle: lc,
		Services:  mockRes,
	}

	handlerOut := NewServiceHandler(params)
	handler := handlerOut.ServiceHandler

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := handler.Start(ctx)
	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond)

	// Create service first
	service := &slim_corev1.Service{
		ObjectMeta: slim_metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "default",
		},
		Spec: slim_corev1.ServiceSpec{
			ClusterIPs: []string{"10.96.0.1", "10.96.0.2"},
		},
	}

	mockRes.sendEvent(resource.Upsert, service)
	time.Sleep(10 * time.Millisecond)

	// Delete service
	mockRes.sendEvent(resource.Delete, service)
	time.Sleep(10 * time.Millisecond)

	// Verify cache is cleared
	handler.mu.RLock()
	defer handler.mu.RUnlock()

	serviceKey := types.NamespacedName{Name: "test-service", Namespace: "default"}
	assert.NotContains(t, handler.ipToService, "10.96.0.1")
	assert.NotContains(t, handler.ipToService, "10.96.0.2")
	assert.NotContains(t, handler.serviceToIPs, serviceKey)
}

func TestServiceHandler_EventHandling_HeadlessService(t *testing.T) {
	mockRes := newMockServiceResource()
	defer mockRes.close()

	lc := &cell.DefaultLifecycle{}
	params := ServiceHandlerParams{
		Lifecycle: lc,
		Services:  mockRes,
	}

	handlerOut := NewServiceHandler(params)
	handler := handlerOut.ServiceHandler

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := handler.Start(ctx)
	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond)

	service := &slim_corev1.Service{
		ObjectMeta: slim_metav1.ObjectMeta{
			Name:      "headless-service",
			Namespace: "default",
		},
		Spec: slim_corev1.ServiceSpec{
			ClusterIPs: []string{"None"},
		},
	}

	mockRes.sendEvent(resource.Upsert, service)
	time.Sleep(10 * time.Millisecond)

	// Verify no IPs are cached for headless service
	handler.mu.RLock()
	defer handler.mu.RUnlock()

	expectedKey := types.NamespacedName{Name: "headless-service", Namespace: "default"}
	assert.NotContains(t, handler.ipToService, "None")
	assert.Contains(t, handler.serviceToIPs, expectedKey)
	assert.Empty(t, handler.serviceToIPs[expectedKey])
}

func TestServiceHandler_Decode_ExistingIP(t *testing.T) {
	mockRes := newMockServiceResource()
	defer mockRes.close()

	lc := &cell.DefaultLifecycle{}
	params := ServiceHandlerParams{
		Lifecycle: lc,
		Services:  mockRes,
	}

	handlerOut := NewServiceHandler(params)
	handler := handlerOut.ServiceHandler

	// Pre-populate cache
	serviceKey := types.NamespacedName{Name: "test-service", Namespace: "default"}
	flowService := &flow.Service{Name: "test-service", Namespace: "default"}
	handler.mu.Lock()
	handler.ipToService["10.96.0.1"] = flowService
	handler.serviceToIPs[serviceKey] = []string{"10.96.0.1"}
	handler.mu.Unlock()

	ip, _ := netip.ParseAddr("10.96.0.1")
	result := handler.Decode(ip)

	require.NotNil(t, result)
	assert.Equal(t, "test-service", result.GetName())
	assert.Equal(t, "default", result.GetNamespace())
	assert.Same(t, flowService, result)
}

func TestServiceHandler_Decode_NonExistingIP(t *testing.T) {
	mockRes := newMockServiceResource()
	defer mockRes.close()

	lc := &cell.DefaultLifecycle{}
	params := ServiceHandlerParams{
		Lifecycle: lc,
		Services:  mockRes,
	}

	handlerOut := NewServiceHandler(params)
	handler := handlerOut.ServiceHandler

	ip, _ := netip.ParseAddr("10.96.0.100")
	result := handler.Decode(ip)

	assert.Nil(t, result)
}

func TestServiceHandler_Decode_IPv6(t *testing.T) {
	mockRes := newMockServiceResource()
	defer mockRes.close()

	lc := &cell.DefaultLifecycle{}
	params := ServiceHandlerParams{
		Lifecycle: lc,
		Services:  mockRes,
	}

	handlerOut := NewServiceHandler(params)
	handler := handlerOut.ServiceHandler

	// Pre-populate cache with IPv6
	serviceKey := types.NamespacedName{Name: "test-service-v6", Namespace: "kube-system"}
	flowService := &flow.Service{Name: "test-service-v6", Namespace: "kube-system"}
	handler.mu.Lock()
	handler.ipToService["fd00::1"] = flowService
	handler.serviceToIPs[serviceKey] = []string{"fd00::1"}
	handler.mu.Unlock()

	ip, _ := netip.ParseAddr("fd00::1")
	result := handler.Decode(ip)

	require.NotNil(t, result)
	assert.Equal(t, "test-service-v6", result.GetName())
	assert.Equal(t, "kube-system", result.GetNamespace())
	assert.Same(t, flowService, result)
}

func TestServiceHandler_SyncEvent(t *testing.T) {
	mockRes := newMockServiceResource()
	defer mockRes.close()

	lc := &cell.DefaultLifecycle{}
	params := ServiceHandlerParams{
		Lifecycle: lc,
		Services:  mockRes,
	}

	handlerOut := NewServiceHandler(params)
	handler := handlerOut.ServiceHandler

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := handler.Start(ctx)
	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond)

	service := &slim_corev1.Service{
		ObjectMeta: slim_metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "default",
		},
		Spec: slim_corev1.ServiceSpec{
			ClusterIPs: []string{"10.96.0.1"},
		},
	}

	// Send sync event (should be ignored)
	mockRes.sendEvent(resource.Sync, service)
	time.Sleep(10 * time.Millisecond)

	// Verify cache is empty
	handler.mu.RLock()
	defer handler.mu.RUnlock()

	assert.Empty(t, handler.ipToService)
	assert.Empty(t, handler.serviceToIPs)
}

func TestServiceHandler_ConcurrentAccess(t *testing.T) {
	mockRes := newMockServiceResource()
	defer mockRes.close()

	lc := &cell.DefaultLifecycle{}
	params := ServiceHandlerParams{
		Lifecycle: lc,
		Services:  mockRes,
	}

	handlerOut := NewServiceHandler(params)
	handler := handlerOut.ServiceHandler

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := handler.Start(ctx)
	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond)

	service := &slim_corev1.Service{
		ObjectMeta: slim_metav1.ObjectMeta{
			Name:      "concurrent-test-service",
			Namespace: "default",
		},
		Spec: slim_corev1.ServiceSpec{
			ClusterIPs: []string{"10.100.0.1"},
		},
	}

	// Test concurrent read/write access
	done := make(chan bool, 2)

	// Goroutine 1: Send event
	go func() {
		mockRes.sendEvent(resource.Upsert, service)
		done <- true
	}()

	// Goroutine 2: Read from cache
	go func() {
		time.Sleep(5 * time.Millisecond)
		ip, _ := netip.ParseAddr("10.100.0.1")
		handler.Decode(ip)
		done <- true
	}()

	// Wait for both goroutines to complete
	<-done
	<-done

	time.Sleep(10 * time.Millisecond)

	// Verify final state
	ip, _ := netip.ParseAddr("10.100.0.1")
	result := handler.Decode(ip)

	require.NotNil(t, result)
	assert.Equal(t, "concurrent-test-service", result.GetName())
	assert.Equal(t, "default", result.GetNamespace())
}

func TestServiceHandler_StopClosesChannel(t *testing.T) {
	mockRes := newMockServiceResource()

	lc := &cell.DefaultLifecycle{}
	params := ServiceHandlerParams{
		Lifecycle: lc,
		Services:  mockRes,
	}

	handlerOut := NewServiceHandler(params)
	handler := handlerOut.ServiceHandler

	ctx, cancel := context.WithCancel(context.Background())

	err := handler.Start(ctx)
	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond)

	// Close the mock resource
	mockRes.close()
	time.Sleep(10 * time.Millisecond)

	// Stop should complete without error
	err = handler.Stop(ctx)
	require.NoError(t, err)

	cancel()
}
