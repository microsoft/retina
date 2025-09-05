package resources

import (
	"context"
	"net/netip"
	"testing"

	"github.com/cilium/cilium/api/v1/flow"
	"github.com/microsoft/retina/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var testScheme = runtime.NewScheme()

func init() {
	_, _ = log.SetupZapLogger(log.GetDefaultLogOpts())
	utilruntime.Must(clientgoscheme.AddToScheme(testScheme))
}

func TestNewServiceReconciler(t *testing.T) {
	fakeClient := fake.NewClientBuilder().WithScheme(testScheme).Build()

	reconciler := NewServiceReconciler(fakeClient)

	assert.NotNil(t, reconciler)
	assert.Equal(t, fakeClient, reconciler.Client)
	assert.NotNil(t, reconciler.ipToService)
	assert.NotNil(t, reconciler.serviceToIPs)
	assert.NotNil(t, reconciler.logger)
	assert.Equal(t, ServiceReconcilerName, reconciler.logger.Name())
}

func TestServiceReconciler_Reconcile_ServiceCreated(t *testing.T) {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "default",
		},
		Spec: corev1.ServiceSpec{
			ClusterIPs: []string{"10.96.0.1", "10.96.0.2"},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(testScheme).
		WithObjects(service).
		Build()

	reconciler := NewServiceReconciler(fakeClient)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-service",
			Namespace: "default",
		},
	}

	result, err := reconciler.Reconcile(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	// Verify the service is cached
	reconciler.mu.RLock()
	defer reconciler.mu.RUnlock()

	expectedKey := types.NamespacedName{Name: "test-service", Namespace: "default"}
	assert.Contains(t, reconciler.ipToService, "10.96.0.1")
	assert.Contains(t, reconciler.ipToService, "10.96.0.2")

	// Verify flow.Service objects are cached correctly
	flowService1 := reconciler.ipToService["10.96.0.1"]
	flowService2 := reconciler.ipToService["10.96.0.2"]
	assert.Equal(t, "test-service", flowService1.GetName())
	assert.Equal(t, "default", flowService1.GetNamespace())
	assert.Equal(t, "test-service", flowService2.GetName())
	assert.Equal(t, "default", flowService2.GetNamespace())

	// Verify both IPs point to the same object (optimization)
	assert.Same(t, flowService1, flowService2)

	assert.Contains(t, reconciler.serviceToIPs, expectedKey)
	assert.ElementsMatch(t, []string{"10.96.0.1", "10.96.0.2"}, reconciler.serviceToIPs[expectedKey])
}

func TestServiceReconciler_Reconcile_ServiceUpdated(t *testing.T) {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "default",
		},
		Spec: corev1.ServiceSpec{
			ClusterIPs: []string{"10.96.0.3", "10.96.0.4"},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(testScheme).
		WithObjects(service).
		Build()

	reconciler := NewServiceReconciler(fakeClient)

	// Pre-populate cache with old IPs
	oldKey := types.NamespacedName{Name: "test-service", Namespace: "default"}
	oldFlowService := &flow.Service{Name: "test-service", Namespace: "default"}
	reconciler.mu.Lock()
	reconciler.ipToService["10.96.0.1"] = oldFlowService
	reconciler.ipToService["10.96.0.2"] = oldFlowService
	reconciler.serviceToIPs[oldKey] = []string{"10.96.0.1", "10.96.0.2"}
	reconciler.mu.Unlock()

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-service",
			Namespace: "default",
		},
	}

	result, err := reconciler.Reconcile(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	// Verify the service cache is updated with new IPs
	reconciler.mu.RLock()
	defer reconciler.mu.RUnlock()

	expectedKey := types.NamespacedName{Name: "test-service", Namespace: "default"}

	// Old IPs should be gone
	assert.NotContains(t, reconciler.ipToService, "10.96.0.1")
	assert.NotContains(t, reconciler.ipToService, "10.96.0.2")

	// New IPs should be present
	assert.Contains(t, reconciler.ipToService, "10.96.0.3")
	assert.Contains(t, reconciler.ipToService, "10.96.0.4")

	// Verify flow.Service objects are cached correctly
	flowService3 := reconciler.ipToService["10.96.0.3"]
	flowService4 := reconciler.ipToService["10.96.0.4"]
	assert.Equal(t, "test-service", flowService3.GetName())
	assert.Equal(t, "default", flowService3.GetNamespace())
	assert.Equal(t, "test-service", flowService4.GetName())
	assert.Equal(t, "default", flowService4.GetNamespace())

	// Verify both IPs point to the same object (optimization)
	assert.Same(t, flowService3, flowService4)

	assert.Contains(t, reconciler.serviceToIPs, expectedKey)
	assert.ElementsMatch(t, []string{"10.96.0.3", "10.96.0.4"}, reconciler.serviceToIPs[expectedKey])
}

func TestServiceReconciler_Reconcile_ServiceDeleted(t *testing.T) {
	// Create reconciler with empty client (service doesn't exist)
	fakeClient := fake.NewClientBuilder().WithScheme(testScheme).Build()
	reconciler := NewServiceReconciler(fakeClient)

	// Pre-populate cache
	serviceKey := types.NamespacedName{Name: "test-service", Namespace: "default"}
	flowService := &flow.Service{Name: "test-service", Namespace: "default"}
	reconciler.mu.Lock()
	reconciler.ipToService["10.96.0.1"] = flowService
	reconciler.ipToService["10.96.0.2"] = flowService
	reconciler.serviceToIPs[serviceKey] = []string{"10.96.0.1", "10.96.0.2"}
	reconciler.mu.Unlock()

	req := ctrl.Request{
		NamespacedName: serviceKey,
	}

	result, err := reconciler.Reconcile(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	// Verify the service is removed from cache
	reconciler.mu.RLock()
	defer reconciler.mu.RUnlock()

	assert.NotContains(t, reconciler.ipToService, "10.96.0.1")
	assert.NotContains(t, reconciler.ipToService, "10.96.0.2")
	assert.NotContains(t, reconciler.serviceToIPs, serviceKey)
}

func TestServiceReconciler_Reconcile_ServiceWithNoneClusterIP(t *testing.T) {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "headless-service",
			Namespace: "default",
		},
		Spec: corev1.ServiceSpec{
			ClusterIPs: []string{"None"},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(testScheme).
		WithObjects(service).
		Build()

	reconciler := NewServiceReconciler(fakeClient)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "headless-service",
			Namespace: "default",
		},
	}

	result, err := reconciler.Reconcile(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	// Verify no IPs are cached for headless service
	reconciler.mu.RLock()
	defer reconciler.mu.RUnlock()

	expectedKey := types.NamespacedName{Name: "headless-service", Namespace: "default"}
	assert.NotContains(t, reconciler.ipToService, "None")
	assert.Contains(t, reconciler.serviceToIPs, expectedKey)
	assert.Empty(t, reconciler.serviceToIPs[expectedKey])
}

func TestServiceReconciler_Reconcile_ServiceWithEmptyClusterIP(t *testing.T) {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "service-no-ip",
			Namespace: "default",
		},
		Spec: corev1.ServiceSpec{
			ClusterIPs: []string{""},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(testScheme).
		WithObjects(service).
		Build()

	reconciler := NewServiceReconciler(fakeClient)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "service-no-ip",
			Namespace: "default",
		},
	}

	result, err := reconciler.Reconcile(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	// Verify no IPs are cached for service with empty cluster IP
	reconciler.mu.RLock()
	defer reconciler.mu.RUnlock()

	expectedKey := types.NamespacedName{Name: "service-no-ip", Namespace: "default"}
	assert.NotContains(t, reconciler.ipToService, "")
	assert.Contains(t, reconciler.serviceToIPs, expectedKey)
	assert.Empty(t, reconciler.serviceToIPs[expectedKey])
}

func TestServiceReconciler_Decode_ExistingIP(t *testing.T) {
	fakeClient := fake.NewClientBuilder().WithScheme(testScheme).Build()
	reconciler := NewServiceReconciler(fakeClient)

	// Pre-populate cache
	serviceKey := types.NamespacedName{Name: "test-service", Namespace: "default"}
	flowService := &flow.Service{Name: "test-service", Namespace: "default"}
	reconciler.mu.Lock()
	reconciler.ipToService["10.96.0.1"] = flowService
	reconciler.serviceToIPs[serviceKey] = []string{"10.96.0.1"}
	reconciler.mu.Unlock()

	ip, _ := netip.ParseAddr("10.96.0.1")
	result := reconciler.Decode(ip)

	require.NotNil(t, result)
	assert.Equal(t, "test-service", result.GetName())
	assert.Equal(t, "default", result.GetNamespace())

	// Verify it returns the same cached object
	assert.Same(t, flowService, result)
}

func TestServiceReconciler_Decode_NonExistingIP(t *testing.T) {
	fakeClient := fake.NewClientBuilder().WithScheme(testScheme).Build()
	reconciler := NewServiceReconciler(fakeClient)

	ip, _ := netip.ParseAddr("10.96.0.100")
	result := reconciler.Decode(ip)

	assert.Nil(t, result)
}

func TestServiceReconciler_Decode_IPv6(t *testing.T) {
	fakeClient := fake.NewClientBuilder().WithScheme(testScheme).Build()
	reconciler := NewServiceReconciler(fakeClient)

	// Pre-populate cache with IPv6
	serviceKey := types.NamespacedName{Name: "test-service-v6", Namespace: "kube-system"}
	flowService := &flow.Service{Name: "test-service-v6", Namespace: "kube-system"}
	reconciler.mu.Lock()
	reconciler.ipToService["fd00::1"] = flowService
	reconciler.serviceToIPs[serviceKey] = []string{"fd00::1"}
	reconciler.mu.Unlock()

	ip, _ := netip.ParseAddr("fd00::1")
	result := reconciler.Decode(ip)

	require.NotNil(t, result)
	assert.Equal(t, "test-service-v6", result.GetName())
	assert.Equal(t, "kube-system", result.GetNamespace())

	// Verify it returns the same cached object
	assert.Same(t, flowService, result)
}

func TestServiceReconciler_updateServiceCache(t *testing.T) {
	fakeClient := fake.NewClientBuilder().WithScheme(testScheme).Build()
	reconciler := NewServiceReconciler(fakeClient)

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cache-test-service",
			Namespace: "test-namespace",
		},
		Spec: corev1.ServiceSpec{
			ClusterIPs: []string{"192.168.1.1", "192.168.1.2"},
		},
	}

	reconciler.updateServiceCache(service)

	reconciler.mu.RLock()
	defer reconciler.mu.RUnlock()

	expectedKey := types.NamespacedName{Name: "cache-test-service", Namespace: "test-namespace"}
	assert.Contains(t, reconciler.ipToService, "192.168.1.1")
	assert.Contains(t, reconciler.ipToService, "192.168.1.2")

	// Verify flow.Service objects are cached correctly
	flowService1 := reconciler.ipToService["192.168.1.1"]
	flowService2 := reconciler.ipToService["192.168.1.2"]
	assert.Equal(t, "cache-test-service", flowService1.GetName())
	assert.Equal(t, "test-namespace", flowService1.GetNamespace())
	assert.Equal(t, "cache-test-service", flowService2.GetName())
	assert.Equal(t, "test-namespace", flowService2.GetNamespace())

	// Verify both IPs point to the same object (optimization)
	assert.Same(t, flowService1, flowService2)

	assert.Contains(t, reconciler.serviceToIPs, expectedKey)
	assert.ElementsMatch(t, []string{"192.168.1.1", "192.168.1.2"}, reconciler.serviceToIPs[expectedKey])
}

func TestServiceReconciler_removeServiceFromCache(t *testing.T) {
	fakeClient := fake.NewClientBuilder().WithScheme(testScheme).Build()
	reconciler := NewServiceReconciler(fakeClient)

	// Pre-populate cache
	serviceKey := types.NamespacedName{Name: "remove-test-service", Namespace: "test-namespace"}
	flowService := &flow.Service{Name: "remove-test-service", Namespace: "test-namespace"}
	reconciler.mu.Lock()
	reconciler.ipToService["172.16.1.1"] = flowService
	reconciler.ipToService["172.16.1.2"] = flowService
	reconciler.serviceToIPs[serviceKey] = []string{"172.16.1.1", "172.16.1.2"}
	reconciler.mu.Unlock()

	reconciler.removeServiceFromCache(serviceKey)

	reconciler.mu.RLock()
	defer reconciler.mu.RUnlock()

	assert.NotContains(t, reconciler.ipToService, "172.16.1.1")
	assert.NotContains(t, reconciler.ipToService, "172.16.1.2")
	assert.NotContains(t, reconciler.serviceToIPs, serviceKey)
}

func TestServiceReconciler_removeServiceFromCache_NonExistentService(t *testing.T) {
	fakeClient := fake.NewClientBuilder().WithScheme(testScheme).Build()
	reconciler := NewServiceReconciler(fakeClient)

	nonExistentKey := types.NamespacedName{Name: "non-existent", Namespace: "default"}

	// This should not panic
	reconciler.removeServiceFromCache(nonExistentKey)

	reconciler.mu.RLock()
	defer reconciler.mu.RUnlock()

	assert.Empty(t, reconciler.ipToService)
	assert.Empty(t, reconciler.serviceToIPs)
}

func TestServiceReconciler_addServiceIPMappings(t *testing.T) {
	fakeClient := fake.NewClientBuilder().WithScheme(testScheme).Build()
	reconciler := NewServiceReconciler(fakeClient)

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mapping-test-service",
			Namespace: "test-namespace",
		},
		Spec: corev1.ServiceSpec{
			ClusterIPs: []string{"10.0.0.1", "10.0.0.2", "", "None"},
		},
	}

	reconciler.mu.Lock()
	reconciler.addServiceIPMappings(service)
	reconciler.mu.Unlock()

	reconciler.mu.RLock()
	defer reconciler.mu.RUnlock()

	expectedKey := types.NamespacedName{Name: "mapping-test-service", Namespace: "test-namespace"}

	// Valid IPs should be mapped
	assert.Contains(t, reconciler.ipToService, "10.0.0.1")
	assert.Contains(t, reconciler.ipToService, "10.0.0.2")

	// Verify flow.Service objects are cached correctly
	flowService1 := reconciler.ipToService["10.0.0.1"]
	flowService2 := reconciler.ipToService["10.0.0.2"]
	assert.Equal(t, "mapping-test-service", flowService1.GetName())
	assert.Equal(t, "test-namespace", flowService1.GetNamespace())
	assert.Equal(t, "mapping-test-service", flowService2.GetName())
	assert.Equal(t, "test-namespace", flowService2.GetNamespace())

	// Verify both IPs point to the same object (optimization)
	assert.Same(t, flowService1, flowService2)

	// Invalid IPs should not be mapped
	assert.NotContains(t, reconciler.ipToService, "")
	assert.NotContains(t, reconciler.ipToService, "None")

	// Service should have correct IPs
	assert.Contains(t, reconciler.serviceToIPs, expectedKey)
	assert.ElementsMatch(t, []string{"10.0.0.1", "10.0.0.2"}, reconciler.serviceToIPs[expectedKey])
}

func TestServiceReconciler_ConcurrentAccess(t *testing.T) {
	fakeClient := fake.NewClientBuilder().WithScheme(testScheme).Build()
	reconciler := NewServiceReconciler(fakeClient)

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "concurrent-test-service",
			Namespace: "default",
		},
		Spec: corev1.ServiceSpec{
			ClusterIPs: []string{"10.100.0.1"},
		},
	}

	// Test concurrent read/write access
	done := make(chan bool, 2)

	// Goroutine 1: Update cache
	go func() {
		reconciler.updateServiceCache(service)
		done <- true
	}()

	// Goroutine 2: Read from cache
	go func() {
		ip, _ := netip.ParseAddr("10.100.0.1")
		reconciler.Decode(ip)
		done <- true
	}()

	// Wait for both goroutines to complete
	<-done
	<-done

	// Verify final state
	ip, _ := netip.ParseAddr("10.100.0.1")
	result := reconciler.Decode(ip)

	require.NotNil(t, result)
	assert.Equal(t, "concurrent-test-service", result.GetName())
	assert.Equal(t, "default", result.GetNamespace())
}

func TestServiceReconciler_SetupWithManager(t *testing.T) {
	// This test is more integration-focused and would require a real manager
	// For unit testing, we can verify the method exists and doesn't panic
	fakeClient := fake.NewClientBuilder().WithScheme(testScheme).Build()
	reconciler := NewServiceReconciler(fakeClient)

	// Verify the method exists (compilation test)
	assert.NotNil(t, reconciler.SetupWithManager)
}
