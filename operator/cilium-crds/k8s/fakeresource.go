package k8s

import (
	"context"

	k8sRuntime "k8s.io/apimachinery/pkg/runtime"

	"github.com/cilium/cilium/pkg/k8s/resource"
)

type fakeresource[T k8sRuntime.Object] struct {
}

func (f *fakeresource[T]) Events(ctx context.Context, opts ...resource.EventsOpt) <-chan resource.Event[T] {
	return make(<-chan resource.Event[T])
}

func (f *fakeresource[T]) Store(ctx context.Context) (resource.Store[T], error) {
	return nil, nil
}

func (f *fakeresource[T]) Observe(ctx context.Context, next func(resource.Event[T]), complete func(error)) {
}
