//go:unit

package ciliumutil

import (
	"context"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	k8sRuntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"

	ciliumv2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	"github.com/cilium/cilium/pkg/k8s/resource"
)

var ErrMockStoreFailure = errors.New("mock store failure")

// ensure all interfaces are implemented
var (
	_ resource.Resource[*ciliumv2.CiliumEndpoint] = NewMockResource[*ciliumv2.CiliumEndpoint](nil)
	_ resource.Store[*ciliumv2.CiliumEndpoint]    = NewMockResource[*ciliumv2.CiliumEndpoint](nil)
)

// MockResource is a mock implementation of resource.Resource AND resource.Store
// It currently only implements the methods used in the endpoint controller
// i.e. Store() and GetByKey()
// plus some helpers to add/remove items from the cache and error on the next call to Store()
type MockResource[T k8sRuntime.Object] struct {
	l                       logrus.FieldLogger
	cache                   map[resource.Key]T
	shouldFailNextStoreCall bool
}

func NewMockResource[T k8sRuntime.Object](l logrus.FieldLogger) *MockResource[T] {
	return &MockResource[T]{
		l:     l,
		cache: make(map[resource.Key]T),
	}
}

func (r *MockResource[T]) Upsert(obj T) {
	r.l.Info("Upsert() called")
	r.cache[resource.NewKey(obj)] = obj
}

func (r *MockResource[T]) Delete(k resource.Key) {
	r.l.Info("Delete() called")
	delete(r.cache, k)
}

// FailOnNextStoreCall will cause the next call to Store() to return an error
func (r *MockResource[T]) FailOnNextStoreCall() {
	r.l.Info("next call to Store() will fail")
	r.shouldFailNextStoreCall = true
}

func (r *MockResource[T]) Observe(_ context.Context, _ func(resource.Event[T]), _ func(error)) {
	r.l.Warn("Observe() called but this is not implemented")
}

func (r *MockResource[T]) Events(_ context.Context, _ ...resource.EventsOpt) <-chan resource.Event[T] {
	r.l.Warn("Events() called but this returns nil because it's not implemented")
	return nil
}

func (r *MockResource[T]) Store(context.Context) (resource.Store[T], error) {
	if r.shouldFailNextStoreCall {
		r.l.Info("Store() failed")
		r.shouldFailNextStoreCall = false
		return nil, ErrMockStoreFailure
	}

	r.l.Info("Store() succeeded")
	return r, nil
}

func (r *MockResource[T]) List() []T {
	r.l.Warn("List() called but this returns nil because it's not implemented")
	return nil
}

func (r *MockResource[T]) IterKeys() resource.KeyIter {
	r.l.Warn("IterKeys() called but this returns nil because it's not implemented")
	return nil
}

func (r *MockResource[T]) Get(obj T) (item T, exists bool, err error) {
	r.l.Warn("Get() called but this returns nil because it's not implemented")
	return obj, false, nil
}

func (r *MockResource[T]) GetByKey(key resource.Key) (item T, exists bool, err error) {
	if _, ok := r.cache[key]; ok {
		r.l.Info("GetByKey() called and found item")
		return r.cache[key], true, nil
	}

	r.l.Info("GetByKey() called and no item found")
	return item, false, nil
}

func (r *MockResource[T]) IndexKeys(_, _ string) ([]string, error) {
	r.l.Warn("IndexKeys() called but this returns nil because it's not implemented")
	return nil, nil
}

func (r *MockResource[T]) ByIndex(_, _ string) ([]T, error) {
	r.l.Warn("ByIndex() called but this returns nil because it's not implemented")
	return nil, nil
}

func (r *MockResource[T]) CacheStore() cache.Store {
	r.l.Warn("CacheStore() called but this returns nil because it's not implemented")
	return nil
}

func (r *MockResource[T]) Release() {
	// Implement the logic required by the Release method or leave it as a stub if it's just for testing/mocking
	r.l.Warn("Release() called but this is a stub implementation")
}
