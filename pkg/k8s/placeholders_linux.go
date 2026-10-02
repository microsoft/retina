package k8s

import (
	"context"

	"github.com/cilium/cilium/pkg/k8s/resource"
	k8sRuntime "k8s.io/apimachinery/pkg/runtime"
)

type fakeresource[T k8sRuntime.Object] struct{}

func (f *fakeresource[T]) Events(_ context.Context, _ ...resource.EventsOpt) <-chan resource.Event[T] {
	return make(<-chan resource.Event[T])
}

func (f *fakeresource[T]) Store(_ context.Context) (resource.Store[T], error) {
	return nil, nil
}

func (f *fakeresource[T]) Observe(context.Context, func(resource.Event[T]), func(error)) {
}

type watcherconfig struct{}

func (w *watcherconfig) K8sNetworkPolicyEnabled() bool        { return false }
func (w *watcherconfig) K8sClusterNetworkPolicyEnabled() bool { return false }

// fakeRestorer is a no-op endpointstate.Restorer (Retina doesn't restore endpoints).
type fakeRestorer struct{}

func (fakeRestorer) WaitForEndpointRestoreWithoutRegeneration(context.Context) error { return nil }
func (fakeRestorer) WaitForEndpointRestore(context.Context) error                    { return nil }
func (fakeRestorer) WaitForInitialPolicy(context.Context) error                      { return nil }
