package resources

import (
	"context"
	"testing"
	"time"

	cid "github.com/cilium/cilium/pkg/identity"
	cilium_api_v2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	"github.com/cilium/cilium/pkg/k8s/resource"
	"github.com/cilium/hive/cell"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// mockIdentityResource is a test implementation of resource.Resource[*cilium_api_v2.CiliumIdentity]
type mockIdentityResource struct {
	eventChan chan resource.Event[*cilium_api_v2.CiliumIdentity]
}

func newMockIdentityResource() *mockIdentityResource {
	return &mockIdentityResource{
		eventChan: make(chan resource.Event[*cilium_api_v2.CiliumIdentity], 100),
	}
}

func (m *mockIdentityResource) Events(context.Context, ...resource.EventsOpt) <-chan resource.Event[*cilium_api_v2.CiliumIdentity] {
	return m.eventChan
}

func (m *mockIdentityResource) Observe(context.Context, func(resource.Event[*cilium_api_v2.CiliumIdentity]), func(error)) {
}

func (m *mockIdentityResource) Store(context.Context) (resource.Store[*cilium_api_v2.CiliumIdentity], error) {
	return nil, nil
}

func (m *mockIdentityResource) sendEvent(kind resource.EventKind, identity *cilium_api_v2.CiliumIdentity) {
	key := resource.Key{}
	if identity != nil {
		key.Name = identity.Name
		key.Namespace = identity.Namespace
	}

	done := make(chan error, 1)
	m.eventChan <- resource.Event[*cilium_api_v2.CiliumIdentity]{
		Kind:   kind,
		Key:    key,
		Object: identity,
		Done:   func(err error) { done <- err },
	}
	// Wait for event to be processed
	<-done
}

func (m *mockIdentityResource) close() {
	close(m.eventChan)
}

func TestNewCiliumIdentityHandler(t *testing.T) {
	mockRes := newMockIdentityResource()
	defer mockRes.close()

	lc := &cell.DefaultLifecycle{}
	params := CiliumIdentityHandlerParams{
		Lifecycle:  lc,
		Identities: mockRes,
	}

	handlerOut := NewCiliumIdentityHandler(params)

	assert.NotNil(t, handlerOut.CiliumIdentityHandler)
	assert.NotNil(t, handlerOut.LabelCache)
	assert.NotNil(t, handlerOut.labelsByIdentityID)
	assert.NotNil(t, handlerOut.logger)
	assert.Equal(t, CiliumIdentityHandlerName, handlerOut.logger.Name())
}

func TestCiliumIdentityHandler_EventHandling_IdentityCreated(t *testing.T) {
	mockRes := newMockIdentityResource()
	defer mockRes.close()

	lc := &cell.DefaultLifecycle{}
	params := CiliumIdentityHandlerParams{
		Lifecycle:  lc,
		Identities: mockRes,
	}

	handlerOut := NewCiliumIdentityHandler(params)
	handler := handlerOut.CiliumIdentityHandler

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := handler.Start(ctx)
	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond)

	identity := &cilium_api_v2.CiliumIdentity{
		ObjectMeta: metav1.ObjectMeta{
			Name: "123",
		},
		SecurityLabels: map[string]string{
			"k8s:app":     "nginx",
			"k8s:version": "1.0",
			"k8s:tier":    "frontend",
		},
	}

	mockRes.sendEvent(resource.Upsert, identity)
	time.Sleep(10 * time.Millisecond)

	// Verify the identity was cached
	expectedID := cid.NumericIdentity(123)
	labels := handler.GetLabelsFromSecurityIdentity(expectedID)
	require.NotNil(t, labels)
	assert.Len(t, labels, 3)
	assert.Contains(t, labels, "k8s:app=nginx")
	assert.Contains(t, labels, "k8s:version=1.0")
	assert.Contains(t, labels, "k8s:tier=frontend")
}

func TestCiliumIdentityHandler_EventHandling_IdentityUpdated(t *testing.T) {
	mockRes := newMockIdentityResource()
	defer mockRes.close()

	lc := &cell.DefaultLifecycle{}
	params := CiliumIdentityHandlerParams{
		Lifecycle:  lc,
		Identities: mockRes,
	}

	handlerOut := NewCiliumIdentityHandler(params)
	handler := handlerOut.CiliumIdentityHandler

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := handler.Start(ctx)
	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond)

	// Initial identity
	identity := &cilium_api_v2.CiliumIdentity{
		ObjectMeta: metav1.ObjectMeta{
			Name: "456",
		},
		SecurityLabels: map[string]string{
			"k8s:app": "nginx",
		},
	}

	mockRes.sendEvent(resource.Upsert, identity)
	time.Sleep(10 * time.Millisecond)

	// Update the identity with new labels
	updatedIdentity := &cilium_api_v2.CiliumIdentity{
		ObjectMeta: metav1.ObjectMeta{
			Name: "456",
		},
		SecurityLabels: map[string]string{
			"k8s:app":     "nginx",
			"k8s:version": "2.0",
		},
	}

	mockRes.sendEvent(resource.Upsert, updatedIdentity)
	time.Sleep(10 * time.Millisecond)

	// Verify the updated labels in cache
	expectedID := cid.NumericIdentity(456)
	labels := handler.GetLabelsFromSecurityIdentity(expectedID)
	require.NotNil(t, labels)
	assert.Len(t, labels, 2)
	assert.Contains(t, labels, "k8s:app=nginx")
	assert.Contains(t, labels, "k8s:version=2.0")
}

func TestCiliumIdentityHandler_EventHandling_IdentityDeleted(t *testing.T) {
	mockRes := newMockIdentityResource()
	defer mockRes.close()

	lc := &cell.DefaultLifecycle{}
	params := CiliumIdentityHandlerParams{
		Lifecycle:  lc,
		Identities: mockRes,
	}

	handlerOut := NewCiliumIdentityHandler(params)
	handler := handlerOut.CiliumIdentityHandler

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := handler.Start(ctx)
	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond)

	// Create identity first
	identity := &cilium_api_v2.CiliumIdentity{
		ObjectMeta: metav1.ObjectMeta{
			Name: "789",
		},
		SecurityLabels: map[string]string{
			"k8s:app": "test",
		},
	}

	mockRes.sendEvent(resource.Upsert, identity)
	time.Sleep(10 * time.Millisecond)

	// Delete the identity
	mockRes.sendEvent(resource.Delete, identity)
	time.Sleep(10 * time.Millisecond)

	// Verify the identity was removed from cache
	expectedID := cid.NumericIdentity(789)
	labels := handler.GetLabelsFromSecurityIdentity(expectedID)
	assert.Nil(t, labels)
}

func TestCiliumIdentityHandler_EventHandling_InvalidIdentityName(t *testing.T) {
	mockRes := newMockIdentityResource()
	defer mockRes.close()

	lc := &cell.DefaultLifecycle{}
	params := CiliumIdentityHandlerParams{
		Lifecycle:  lc,
		Identities: mockRes,
	}

	handlerOut := NewCiliumIdentityHandler(params)
	handler := handlerOut.CiliumIdentityHandler

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := handler.Start(ctx)
	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond)

	identity := &cilium_api_v2.CiliumIdentity{
		ObjectMeta: metav1.ObjectMeta{
			Name: "invalid-name",
		},
		SecurityLabels: map[string]string{
			"k8s:app": "test",
		},
	}

	// This should not panic, just log an error
	mockRes.sendEvent(resource.Upsert, identity)
	time.Sleep(10 * time.Millisecond)

	// Cache should be empty
	handler.mu.RLock()
	defer handler.mu.RUnlock()
	assert.Empty(t, handler.labelsByIdentityID)
}

func TestCiliumIdentityHandler_GetLabelsFromSecurityIdentity(t *testing.T) {
	mockRes := newMockIdentityResource()
	defer mockRes.close()

	lc := &cell.DefaultLifecycle{}
	params := CiliumIdentityHandlerParams{
		Lifecycle:  lc,
		Identities: mockRes,
	}

	handlerOut := NewCiliumIdentityHandler(params)
	handler := handlerOut.CiliumIdentityHandler

	// Test case 1: Identity exists in cache
	expectedID := cid.NumericIdentity(100)
	expectedLabels := []string{"k8s:app=nginx", "k8s:version=1.0"}
	handler.mu.Lock()
	handler.labelsByIdentityID[expectedID] = expectedLabels
	handler.mu.Unlock()

	labels := handler.GetLabelsFromSecurityIdentity(expectedID)
	assert.Equal(t, expectedLabels, labels)

	// Test case 2: Identity does not exist in cache
	nonExistentID := cid.NumericIdentity(404)
	labels = handler.GetLabelsFromSecurityIdentity(nonExistentID)
	assert.Nil(t, labels)
}

func TestCiliumIdentityHandler_updateIdentityCache(t *testing.T) {
	mockRes := newMockIdentityResource()
	defer mockRes.close()

	lc := &cell.DefaultLifecycle{}
	params := CiliumIdentityHandlerParams{
		Lifecycle:  lc,
		Identities: mockRes,
	}

	handlerOut := NewCiliumIdentityHandler(params)
	handler := handlerOut.CiliumIdentityHandler

	identity := &cilium_api_v2.CiliumIdentity{
		ObjectMeta: metav1.ObjectMeta{
			Name: "200",
		},
		SecurityLabels: map[string]string{
			"k8s:app":  "web",
			"k8s:tier": "frontend",
			"security": "restricted",
		},
	}

	err := handler.updateIdentityCache(identity)
	require.NoError(t, err)

	// Verify the cache was updated
	expectedID := cid.NumericIdentity(200)
	labels := handler.GetLabelsFromSecurityIdentity(expectedID)
	require.NotNil(t, labels)
	assert.Len(t, labels, 3)
	assert.Contains(t, labels, "k8s:app=web")
	assert.Contains(t, labels, "k8s:tier=frontend")
	assert.Contains(t, labels, "security=restricted")
}

func TestCiliumIdentityHandler_updateIdentityCache_InvalidName(t *testing.T) {
	mockRes := newMockIdentityResource()
	defer mockRes.close()

	lc := &cell.DefaultLifecycle{}
	params := CiliumIdentityHandlerParams{
		Lifecycle:  lc,
		Identities: mockRes,
	}

	handlerOut := NewCiliumIdentityHandler(params)
	handler := handlerOut.CiliumIdentityHandler

	identity := &cilium_api_v2.CiliumIdentity{
		ObjectMeta: metav1.ObjectMeta{
			Name: "not-a-number",
		},
		SecurityLabels: map[string]string{
			"k8s:app": "test",
		},
	}

	err := handler.updateIdentityCache(identity)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid identity name")
}

func TestCiliumIdentityHandler_removeIdentityFromCache(t *testing.T) {
	mockRes := newMockIdentityResource()
	defer mockRes.close()

	lc := &cell.DefaultLifecycle{}
	params := CiliumIdentityHandlerParams{
		Lifecycle:  lc,
		Identities: mockRes,
	}

	handlerOut := NewCiliumIdentityHandler(params)
	handler := handlerOut.CiliumIdentityHandler

	// Pre-populate cache
	expectedID := cid.NumericIdentity(300)
	handler.mu.Lock()
	handler.labelsByIdentityID[expectedID] = []string{"k8s:app=test"}
	handler.mu.Unlock()

	// Test removal
	handler.removeIdentityFromCache("300")

	// Verify removal
	labels := handler.GetLabelsFromSecurityIdentity(expectedID)
	assert.Nil(t, labels)
}

func TestCiliumIdentityHandler_removeIdentityFromCache_InvalidName(_ *testing.T) {
	mockRes := newMockIdentityResource()
	defer mockRes.close()

	lc := &cell.DefaultLifecycle{}
	params := CiliumIdentityHandlerParams{
		Lifecycle:  lc,
		Identities: mockRes,
	}

	handlerOut := NewCiliumIdentityHandler(params)
	handler := handlerOut.CiliumIdentityHandler

	// This should not panic even with invalid name
	handler.removeIdentityFromCache("invalid-name")

	// No assertions needed, just ensure it doesn't panic
}

func TestCiliumIdentityHandler_removeIdentityFromCache_NotInCache(_ *testing.T) {
	mockRes := newMockIdentityResource()
	defer mockRes.close()

	lc := &cell.DefaultLifecycle{}
	params := CiliumIdentityHandlerParams{
		Lifecycle:  lc,
		Identities: mockRes,
	}

	handlerOut := NewCiliumIdentityHandler(params)
	handler := handlerOut.CiliumIdentityHandler

	// Try to remove identity that doesn't exist in cache
	handler.removeIdentityFromCache("404")

	// No assertions needed, just ensure it doesn't panic
}

func TestCiliumIdentityHandler_SyncEvent(t *testing.T) {
	mockRes := newMockIdentityResource()
	defer mockRes.close()

	lc := &cell.DefaultLifecycle{}
	params := CiliumIdentityHandlerParams{
		Lifecycle:  lc,
		Identities: mockRes,
	}

	handlerOut := NewCiliumIdentityHandler(params)
	handler := handlerOut.CiliumIdentityHandler

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := handler.Start(ctx)
	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond)

	identity := &cilium_api_v2.CiliumIdentity{
		ObjectMeta: metav1.ObjectMeta{
			Name: "123",
		},
		SecurityLabels: map[string]string{
			"k8s:app": "test",
		},
	}

	// Send sync event (should be ignored)
	mockRes.sendEvent(resource.Sync, identity)
	time.Sleep(10 * time.Millisecond)

	// Verify cache is empty
	handler.mu.RLock()
	defer handler.mu.RUnlock()

	assert.Empty(t, handler.labelsByIdentityID)
}

func TestCiliumIdentityHandler_ConcurrentAccess(t *testing.T) {
	mockRes := newMockIdentityResource()
	defer mockRes.close()

	lc := &cell.DefaultLifecycle{}
	params := CiliumIdentityHandlerParams{
		Lifecycle:  lc,
		Identities: mockRes,
	}

	handlerOut := NewCiliumIdentityHandler(params)
	handler := handlerOut.CiliumIdentityHandler

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := handler.Start(ctx)
	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond)

	identity := &cilium_api_v2.CiliumIdentity{
		ObjectMeta: metav1.ObjectMeta{
			Name: "500",
		},
		SecurityLabels: map[string]string{
			"k8s:app":     "test",
			"k8s:version": "1.0",
		},
	}

	// Test concurrent read/write access
	done := make(chan bool, 2)

	// Goroutine 1: Send event
	go func() {
		mockRes.sendEvent(resource.Upsert, identity)
		done <- true
	}()

	// Goroutine 2: Read from cache
	go func() {
		time.Sleep(5 * time.Millisecond)
		identityID := cid.NumericIdentity(500)
		handler.GetLabelsFromSecurityIdentity(identityID)
		done <- true
	}()

	// Wait for both goroutines to complete
	<-done
	<-done

	time.Sleep(10 * time.Millisecond)

	// Verify final state
	identityID := cid.NumericIdentity(500)
	labels := handler.GetLabelsFromSecurityIdentity(identityID)
	require.NotNil(t, labels)
	assert.Len(t, labels, 2)
	assert.Contains(t, labels, "k8s:app=test")
	assert.Contains(t, labels, "k8s:version=1.0")
}

func TestCiliumIdentityHandler_StopClosesChannel(t *testing.T) {
	mockRes := newMockIdentityResource()

	lc := &cell.DefaultLifecycle{}
	params := CiliumIdentityHandlerParams{
		Lifecycle:  lc,
		Identities: mockRes,
	}

	handlerOut := NewCiliumIdentityHandler(params)
	handler := handlerOut.CiliumIdentityHandler

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
