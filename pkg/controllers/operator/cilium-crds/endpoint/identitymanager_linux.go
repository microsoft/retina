package endpointcontroller

import (
	"context"
	"log/slog"
	"time"

	"github.com/pkg/errors"

	"github.com/cilium/cilium/pkg/identity"
	"github.com/cilium/cilium/pkg/identity/cache"
	"github.com/cilium/cilium/pkg/k8s/client/clientset/versioned"
	"github.com/cilium/cilium/pkg/labels"
)

// Default timeout for identity allocation operations
const identityAllocatorTimeout = 2 * time.Minute

// ErrIdentityNotFound is returned in operator-managed mode when cilium-operator has not yet created
// a CiliumIdentity for the requested labels.
var ErrIdentityNotFound = errors.New("identity not found")

// IdentityManager is analogous to Cilium Daemon's identity allocation.
// Cilium has an IPCacche holding IP to Identity mapping.
// In IPCache.InjectLabels(), IPCacche is told of IPs which have been updated.
// Within this function, identities are allocated/released via CachingIdentityAllocator.
type IdentityManager struct {
	l *slog.Logger
	// alloc is the CachingIdentityAllocator which helps in:
	// - allocating/releasing identities (maintaining reference counts and creating CRDs)
	// - syncing identity "keys", preventing them from being garbage collected
	// The struct performs a bit more than is needed including:
	// - logic for local identities (e.g. node-local CIDR identity), which we do not use
	// - a go routine for notifications on identity changes
	alloc *cache.CachingIdentityAllocator
	// labelIdentities maps sorted labels (via labels.Labels.String()) to allocated identity
	labelIdentities map[string]identity.NumericIdentity
	// operatorManaged means cilium-operator creates CiliumIdentities: they are only looked up,
	// no references are held and labelIdentities is unused.
	operatorManaged bool
}

// owner is a no-op implementation of IdentityAllocatorOwner
type owner struct{}

// UpdateIdentities is a callback when identities are updated
func (o *owner) UpdateIdentities(_, _ identity.IdentityMap) <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}

// GetNodeSuffix() is only used for KVStoreBackend (we use CRDBackend)
func (o *owner) GetNodeSuffix() string {
	return ""
}

// NewIdentityManager creates the identity allocator. With operatorManaged, identities are never created:
// they are looked up, and ErrIdentityNotFound is returned until cilium-operator creates them.
func NewIdentityManager(ctx context.Context, l *slog.Logger, client versioned.Interface, operatorManaged bool) (*IdentityManager, error) {
	// Configure the allocator with a reasonable timeout for identity operations
	allocConfig := cache.AllocatorConfig{
		EnableOperatorManageCIDs: operatorManaged,
		Timeout:                  identityAllocatorTimeout,
		SyncInterval:             1 * time.Hour,
	}

	im := &IdentityManager{
		l:               l.With("component", "identitymanager"),
		alloc:           cache.NewCachingIdentityAllocator(l, &owner{}, allocConfig),
		labelIdentities: make(map[string]identity.NumericIdentity),
		operatorManaged: operatorManaged,
	}

	im.l.Info("initializing identity allocator", "operatorManaged", operatorManaged)
	initCh := im.alloc.InitIdentityAllocator(client, nil)

	// Wait for the identity allocator to be initialized (backend is created)
	// Note: This doesn't wait for the full cache sync - that happens on first allocation
	select {
	case <-initCh:
		im.l.Info("identity allocator initialized successfully")
	case <-ctx.Done():
		return nil, errors.Wrap(ctx.Err(), "context cancelled while waiting for identity allocator initialization")
	}

	return im, nil
}

// DecrementReference modifies the corresponding identity's reference count in the allocator's store.
// For proper garbage collection of stale identities, this must be called exactly once per deleted/relabeled Pod.
// Whenever reference count is not 0, then the identity will exist in the local store, and syncLocalKeys() will make sure it exists.
func (im *IdentityManager) DecrementReference(ctx context.Context, lbls labels.Labels) {
	sortedLabels := lbls.String()
	if im.operatorManaged {
		// No reference is held. cilium-operator owns the identity's lifecycle.
		return
	}

	id, ok := im.labelIdentities[sortedLabels]
	if !ok {
		im.l.Warn("expected identity for labels", "labels", sortedLabels)
		return
	}

	idObj := im.alloc.LookupIdentityByID(ctx, id)
	if idObj == nil {
		// The allocator no longer knows this identity, so there is nothing left to release.
		// Drop our mapping too, otherwise it would be kept for the lifetime of the process.
		im.l.Warn("expected identity for id", "identity", id)
		delete(im.labelIdentities, sortedLabels)
		return
	}

	// modifies the reference count for the identity.
	// If reference count reaches 0, the allocator's store will release the key, meaning identitygc will be able to work,
	// since syncLocalKeys() will no longer make sure the identity exists.
	// notifyOwner=false because no need to notify owner (via UpdateIdentities callback).
	// Since Release() is a local operation (deleting CiliumIdentity happens in identitygc cell),
	// it does not make sense to pass a separate context with a kvstore timeout.
	released, err := im.alloc.Release(ctx, idObj, false)
	if err != nil {
		// possible errors are
		// 1. ctx cancelled (in which case, hive is shutting down)
		// 2. identity not found in localKeys cache (nothing to worry about, and GC on CiliumIdentities will work as expected)
		im.l.Warn("error while releasing previously allocated identity",
			"error", err,
			"identity", idObj,
			"identityLabels", idObj.Labels,
		)
	}

	if !released || err != nil {
		return
	}

	im.l.Info("released identity due to no more references",
		"identity", idObj,
		"identityLabels", idObj.Labels,
	)

	delete(im.labelIdentities, sortedLabels)
}

// GetIdentityAndIncrementReference will create/get an identity ID and increment the reference count in the allocator's store.
// For proper garbage collection of stale identities, this must be called exactly once per created/relabeled Pod.
// Whenever reference count is not 0, then the identity will exist in the local store, and syncLocalKeys() will make sure it exists.
// In operator-managed mode the identity is only looked up (no reference is held), and ErrIdentityNotFound is
// returned if cilium-operator has not created it yet.
func (im *IdentityManager) GetIdentityAndIncrementReference(ctx context.Context, lbls labels.Labels) (int64, error) {
	if im.operatorManaged {
		return im.lookupIdentity(ctx, lbls)
	}

	// notifyOwner=false because no need to notify owner (via UpdateIdentities callback).
	// oldNID=identity.InvalidIdentity would only be used for local identities (e.g. node-local CIDR identity), which we don't use.
	// Since this operation will create the CiliumIdentity if needed,
	// pass in a context that completes either once ctx is done or identity allocator timeout is reached.
	allocateCtx, cancel := context.WithTimeout(ctx, identityAllocatorTimeout)
	defer cancel()
	idObj, _, err := im.alloc.AllocateIdentity(allocateCtx, lbls, false, identity.InvalidIdentity)
	if err != nil {
		return -1, errors.Wrap(err, "failed to allocate identity")
	}

	// info-level logging occurs within Allocator for reusing or allocating a new identity/global key

	im.labelIdentities[lbls.String()] = idObj.ID

	return int64(idObj.ID), nil
}

func (im *IdentityManager) lookupIdentity(ctx context.Context, lbls labels.Labels) (int64, error) {
	// LookupIdentity returns nil until the initial CiliumIdentity informer sync completes.
	if err := im.alloc.WaitForInitialGlobalIdentities(ctx); err != nil {
		return -1, errors.Wrap(err, "failed waiting for initial identity sync")
	}

	idObj := im.alloc.LookupIdentity(ctx, lbls)
	if idObj == nil {
		return -1, errors.Wrapf(ErrIdentityNotFound, "labels %s", lbls.String())
	}

	return int64(idObj.ID), nil
}
