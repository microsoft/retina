package k8s

import (
	"context"
	"github.com/cilium/cilium/api/v1/models"
	"github.com/cilium/cilium/pkg/container/set"
	"github.com/cilium/cilium/pkg/identity"
	ipcachetypes "github.com/cilium/cilium/pkg/ipcache/types"
	"github.com/cilium/cilium/pkg/k8s/resource"
	"github.com/cilium/cilium/pkg/policy"
	"github.com/cilium/cilium/pkg/policy/api"
	policytypes "github.com/cilium/cilium/pkg/policy/types"
	cilium "github.com/cilium/proxy/go/cilium/api"
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

// NoOpPolicyRepository is a no-op implementation of the PolicyRepository interface.
type NoOpPolicyRepository struct{}

func (n *NoOpPolicyRepository) BumpRevision() uint64 {
	return 0
}

func (n *NoOpPolicyRepository) GetAuthTypes(identity.NumericIdentity, identity.NumericIdentity) policy.AuthTypes {
	return policy.AuthTypes{}
}

func (n *NoOpPolicyRepository) GetEnvoyHTTPRules(*api.L7Rules, string) (*cilium.HttpNetworkPolicyRules, bool) {
	return nil, false
}

func (n *NoOpPolicyRepository) GetSelectorPolicy(*identity.Identity, uint64, policy.GetPolicyStatistics, uint64) (policy.SelectorPolicy, uint64, error) {
	return nil, 0, nil
}

func (n *NoOpPolicyRepository) GetRevision() uint64 {
	return 0
}

func (n *NoOpPolicyRepository) GetRulesList() *models.Policy {
	return &models.Policy{}
}

func (n *NoOpPolicyRepository) GetSelectorCache() *policy.SelectorCache {
	return nil
}

func (n *NoOpPolicyRepository) GetSubjectSelectorCache() *policy.SelectorCache {
	return nil
}

func (n *NoOpPolicyRepository) Iterate(func(*policytypes.PolicyEntry)) {}

func (n *NoOpPolicyRepository) ReplaceByResource(
	_ policytypes.PolicyEntries, _ ipcachetypes.ResourceID,
) (affectedIDs *set.Set[identity.NumericIdentity], rev uint64, oldRevCnt int) {
	return nil, 0, 0
}

func (n *NoOpPolicyRepository) Search() (entries policytypes.PolicyEntries, rev uint64) {
	return nil, 0
}

func (n *NoOpPolicyRepository) GetPolicySnapshot() map[identity.NumericIdentity]policy.SelectorPolicy {
	return nil
}

// fakeRestorer is a no-op endpointstate.Restorer (Retina doesn't restore endpoints).
type fakeRestorer struct{}

func (fakeRestorer) WaitForEndpointRestoreWithoutRegeneration(context.Context) error { return nil }
func (fakeRestorer) WaitForEndpointRestore(context.Context) error                    { return nil }
func (fakeRestorer) WaitForInitialPolicy(context.Context) error                      { return nil }

