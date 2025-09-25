package resources

import (
	"context"
	"testing"

	cid "github.com/cilium/cilium/pkg/identity"
	v2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	"github.com/microsoft/retina/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

var ciliumTestScheme = runtime.NewScheme()

func init() {
	_, _ = log.SetupZapLogger(log.GetDefaultLogOpts())
	utilruntime.Must(clientgoscheme.AddToScheme(ciliumTestScheme))
	utilruntime.Must(v2.AddToScheme(ciliumTestScheme))
}

func TestNewCiliumIdentityReconciler(t *testing.T) {
	fakeClient := fake.NewClientBuilder().WithScheme(ciliumTestScheme).Build()

	reconcilerOut := NewCiliumIdentityReconciler(fakeClient)

	assert.NotNil(t, reconcilerOut.CiliumIdentityReconciler)
	assert.NotNil(t, reconcilerOut.LabelCache)
	assert.Equal(t, fakeClient, reconcilerOut.Client)
	assert.NotNil(t, reconcilerOut.labelsByIdentityID)
	assert.NotNil(t, reconcilerOut.logger)
	assert.Equal(t, CiliumIdentityReconcilerName, reconcilerOut.logger.Name())
}

func TestCiliumIdentityReconciler_Reconcile_IdentityCreated(t *testing.T) {
	identity := &v2.CiliumIdentity{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "123",
			Namespace: "",
		},
		SecurityLabels: map[string]string{
			"k8s:app":     "nginx",
			"k8s:version": "1.0",
			"k8s:tier":    "frontend",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(ciliumTestScheme).
		WithObjects(identity).
		Build()

	reconciler := NewCiliumIdentityReconciler(fakeClient)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "123",
			Namespace: "",
		},
	}

	ctx := context.Background()
	result, err := reconciler.Reconcile(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	// Verify the identity was cached
	expectedID := cid.NumericIdentity(123)
	labels := reconciler.LabelCache.GetLabelsFromSecurityIdentity(expectedID)
	require.NotNil(t, labels)
	assert.Len(t, labels, 3)
	assert.Contains(t, labels, "k8s:app=nginx")
	assert.Contains(t, labels, "k8s:version=1.0")
	assert.Contains(t, labels, "k8s:tier=frontend")
}

func TestCiliumIdentityReconciler_Reconcile_IdentityUpdated(t *testing.T) {
	identity := &v2.CiliumIdentity{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "456",
			Namespace: "",
		},
		SecurityLabels: map[string]string{
			"k8s:app": "nginx",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(ciliumTestScheme).
		WithObjects(identity).
		Build()

	reconciler := NewCiliumIdentityReconciler(fakeClient)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "456",
			Namespace: "",
		},
	}

	ctx := context.Background()

	// First reconcile to add the identity
	_, err := reconciler.Reconcile(ctx, req)
	require.NoError(t, err)

	// Update the identity with new labels
	identity.SecurityLabels = map[string]string{
		"k8s:app":     "nginx",
		"k8s:version": "2.0",
	}
	err = fakeClient.Update(ctx, identity)
	require.NoError(t, err)

	// Second reconcile to update the identity
	result, err := reconciler.Reconcile(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	// Verify the updated labels in cache
	expectedID := cid.NumericIdentity(456)
	labels := reconciler.LabelCache.GetLabelsFromSecurityIdentity(expectedID)
	require.NotNil(t, labels)
	assert.Len(t, labels, 2)
	assert.Contains(t, labels, "k8s:app=nginx")
	assert.Contains(t, labels, "k8s:version=2.0")
}

func TestCiliumIdentityReconciler_Reconcile_IdentityNotFound(t *testing.T) {
	fakeClient := fake.NewClientBuilder().WithScheme(ciliumTestScheme).Build()
	reconciler := NewCiliumIdentityReconciler(fakeClient)

	// Pre-populate the cache with an identity
	expectedID := cid.NumericIdentity(789)
	reconciler.labelsByIdentityID[expectedID] = []string{"k8s:app=test"}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "789",
			Namespace: "",
		},
	}

	ctx := context.Background()
	result, err := reconciler.Reconcile(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	// Verify the identity was removed from cache
	labels := reconciler.LabelCache.GetLabelsFromSecurityIdentity(expectedID)
	assert.Nil(t, labels)
}

func TestCiliumIdentityReconciler_Reconcile_InvalidIdentityName(t *testing.T) {
	identity := &v2.CiliumIdentity{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "invalid-name",
			Namespace: "",
		},
		SecurityLabels: map[string]string{
			"k8s:app": "test",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(ciliumTestScheme).
		WithObjects(identity).
		Build()

	reconciler := NewCiliumIdentityReconciler(fakeClient)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "invalid-name",
			Namespace: "",
		},
	}

	ctx := context.Background()
	result, err := reconciler.Reconcile(ctx, req)

	require.Error(t, err)
	assert.Equal(t, ctrl.Result{}, result)
	assert.Contains(t, err.Error(), "invalid identity name")
}

func TestCiliumIdentityReconciler_GetLabelsFromSecurityIdentity(t *testing.T) {
	fakeClient := fake.NewClientBuilder().WithScheme(ciliumTestScheme).Build()
	reconciler := NewCiliumIdentityReconciler(fakeClient)

	// Test case 1: Identity exists in cache
	expectedID := cid.NumericIdentity(100)
	expectedLabels := []string{"k8s:app=nginx", "k8s:version=1.0"}
	reconciler.labelsByIdentityID[expectedID] = expectedLabels

	labels := reconciler.LabelCache.GetLabelsFromSecurityIdentity(expectedID)
	assert.Equal(t, expectedLabels, labels)

	// Test case 2: Identity does not exist in cache
	nonExistentID := cid.NumericIdentity(404)
	labels = reconciler.LabelCache.GetLabelsFromSecurityIdentity(nonExistentID)
	assert.Nil(t, labels)
}

func TestCiliumIdentityReconciler_updateIdentityCache(t *testing.T) {
	fakeClient := fake.NewClientBuilder().WithScheme(ciliumTestScheme).Build()
	reconciler := NewCiliumIdentityReconciler(fakeClient)

	identity := &v2.CiliumIdentity{
		ObjectMeta: metav1.ObjectMeta{
			Name: "200",
		},
		SecurityLabels: map[string]string{
			"k8s:app":  "web",
			"k8s:tier": "frontend",
			"security": "restricted",
		},
	}

	result, err := reconciler.updateIdentityCache(identity)

	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	// Verify the cache was updated
	expectedID := cid.NumericIdentity(200)
	labels := reconciler.LabelCache.GetLabelsFromSecurityIdentity(expectedID)
	require.NotNil(t, labels)
	assert.Len(t, labels, 3)
	assert.Contains(t, labels, "k8s:app=web")
	assert.Contains(t, labels, "k8s:tier=frontend")
	assert.Contains(t, labels, "security=restricted")
}

func TestCiliumIdentityReconciler_updateIdentityCache_InvalidName(t *testing.T) {
	fakeClient := fake.NewClientBuilder().WithScheme(ciliumTestScheme).Build()
	reconciler := NewCiliumIdentityReconciler(fakeClient)

	identity := &v2.CiliumIdentity{
		ObjectMeta: metav1.ObjectMeta{
			Name: "not-a-number",
		},
		SecurityLabels: map[string]string{
			"k8s:app": "test",
		},
	}

	result, err := reconciler.updateIdentityCache(identity)

	require.Error(t, err)
	assert.Equal(t, ctrl.Result{}, result)
	assert.Contains(t, err.Error(), "invalid identity name")
}

func TestCiliumIdentityReconciler_removeIdentityFromCache(t *testing.T) {
	fakeClient := fake.NewClientBuilder().WithScheme(ciliumTestScheme).Build()
	reconciler := NewCiliumIdentityReconciler(fakeClient)

	// Pre-populate cache
	expectedID := cid.NumericIdentity(300)
	reconciler.labelsByIdentityID[expectedID] = []string{"k8s:app=test"}

	// Test removal
	objectKey := types.NamespacedName{Name: "300"}
	reconciler.removeIdentityFromCache(objectKey)

	// Verify removal
	labels := reconciler.LabelCache.GetLabelsFromSecurityIdentity(expectedID)
	assert.Nil(t, labels)
}

func TestCiliumIdentityReconciler_removeIdentityFromCache_InvalidName(*testing.T) {
	fakeClient := fake.NewClientBuilder().WithScheme(ciliumTestScheme).Build()
	reconciler := NewCiliumIdentityReconciler(fakeClient)

	// This should not panic even with invalid name
	objectKey := types.NamespacedName{Name: "invalid-name"}
	reconciler.removeIdentityFromCache(objectKey)

	// No assertions needed, just ensure it doesn't panic
}

func TestCiliumIdentityReconciler_removeIdentityFromCache_NotInCache(*testing.T) {
	fakeClient := fake.NewClientBuilder().WithScheme(ciliumTestScheme).Build()
	reconciler := NewCiliumIdentityReconciler(fakeClient)

	// Try to remove identity that doesn't exist in cache
	objectKey := types.NamespacedName{Name: "404"}
	reconciler.removeIdentityFromCache(objectKey)

	// No assertions needed, just ensure it doesn't panic
}

func Test_identityChangedPredicate_CreateFunc(t *testing.T) {
	predicate := identityChangedPredicate()

	createEvent := event.CreateEvent{
		Object: &v2.CiliumIdentity{},
	}

	result := predicate.Create(createEvent)
	assert.True(t, result)
}

func Test_identityChangedPredicate_UpdateFunc(t *testing.T) {
	predicate := identityChangedPredicate()

	oldIdentity := &v2.CiliumIdentity{
		SecurityLabels: map[string]string{
			"k8s:app": "nginx",
		},
	}

	// Test case 1: Labels changed
	newIdentityChanged := &v2.CiliumIdentity{
		SecurityLabels: map[string]string{
			"k8s:app":     "nginx",
			"k8s:version": "2.0",
		},
	}

	updateEvent := event.UpdateEvent{
		ObjectOld: oldIdentity,
		ObjectNew: newIdentityChanged,
	}

	result := predicate.Update(updateEvent)
	assert.True(t, result)

	// Test case 2: Labels unchanged
	newIdentityUnchanged := &v2.CiliumIdentity{
		SecurityLabels: map[string]string{
			"k8s:app": "nginx",
		},
	}

	updateEvent = event.UpdateEvent{
		ObjectOld: oldIdentity,
		ObjectNew: newIdentityUnchanged,
	}

	result = predicate.Update(updateEvent)
	assert.False(t, result)

	// Test case 3: Invalid old object type
	updateEvent = event.UpdateEvent{
		ObjectOld: &v2.CiliumEndpoint{},
		ObjectNew: newIdentityChanged,
	}

	result = predicate.Update(updateEvent)
	assert.False(t, result)

	// Test case 4: Invalid new object type
	updateEvent = event.UpdateEvent{
		ObjectOld: oldIdentity,
		ObjectNew: &v2.CiliumEndpoint{},
	}

	result = predicate.Update(updateEvent)
	assert.False(t, result)
}

func Test_identityChangedPredicate_DeleteFunc(t *testing.T) {
	predicate := identityChangedPredicate()

	deleteEvent := event.DeleteEvent{
		Object: &v2.CiliumIdentity{},
	}

	result := predicate.Delete(deleteEvent)
	assert.True(t, result)
}

func Test_identityChangedPredicate_GenericFunc(t *testing.T) {
	predicate := identityChangedPredicate()

	genericEvent := event.GenericEvent{
		Object: &v2.CiliumIdentity{},
	}

	result := predicate.Generic(genericEvent)
	assert.False(t, result)
}

func Test_mapsEqual(t *testing.T) {
	// Test case 1: Equal maps
	map1 := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}
	map2 := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}
	assert.True(t, mapsEqual(map1, map2))

	// Test case 2: Different lengths
	map3 := map[string]string{
		"key1": "value1",
	}
	assert.False(t, mapsEqual(map1, map3))

	// Test case 3: Same keys, different values
	map4 := map[string]string{
		"key1": "value1",
		"key2": "different_value",
	}
	assert.False(t, mapsEqual(map1, map4))

	// Test case 4: Different keys
	map5 := map[string]string{
		"key1": "value1",
		"key3": "value2",
	}
	assert.False(t, mapsEqual(map1, map5))

	// Test case 5: Empty maps
	map6 := map[string]string{}
	map7 := map[string]string{}
	assert.True(t, mapsEqual(map6, map7))

	// Test case 6: One empty, one not
	assert.False(t, mapsEqual(map1, map6))
}

func TestCiliumIdentityReconciler_ConcurrentAccess(t *testing.T) {
	fakeClient := fake.NewClientBuilder().WithScheme(ciliumTestScheme).Build()
	reconciler := NewCiliumIdentityReconciler(fakeClient)

	// Test concurrent read/write access to the cache
	identityID := cid.NumericIdentity(500)
	labels := []string{"k8s:app=test", "k8s:version=1.0"}

	// Simulate concurrent access
	done := make(chan bool, 2)

	// Goroutine 1: Writing to cache
	go func() {
		reconciler.labelsByIdentityID[identityID] = labels
		done <- true
	}()

	// Goroutine 2: Reading from cache
	go func() {
		_ = reconciler.LabelCache.GetLabelsFromSecurityIdentity(identityID)
		done <- true
	}()

	// Wait for both goroutines to complete
	<-done
	<-done

	// Verify final state
	finalLabels := reconciler.LabelCache.GetLabelsFromSecurityIdentity(identityID)
	assert.Equal(t, labels, finalLabels)
}
