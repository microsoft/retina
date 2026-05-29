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

func NewIdentityManager(ctx context.Context, l *slog.Logger, client versioned.Interface) (*IdentityManager, error) {
	// Configure the allocator with a reasonable timeout for identity operations
	allocConfig := cache.AllocatorConfig{
		Timeout:      identityAllocatorTimeout,
		SyncInterval: 1 * time.Hour,
	}

	im := &IdentityManager{
		l:               l.With("component", "identitymanager"),
		alloc:           cache.NewCachingIdentityAllocator(l, &owner{}, allocConfig),
		labelIdentities: make(map[string]identity.NumericIdentity),
	}

	im.l.Info("initializing identity allocator")
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
	id, ok := im.labelIdentities[sortedLabels]
	if !ok {
		im.l.Warn("expected identity for labels", "labels", sortedLabels)
		return
	}

	idObj := im.alloc.LookupIdentityByID(ctx, id)
	if idObj == nil {
		im.l.Warn("expected identity for id", "identity", id)
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
func (im *IdentityManager) GetIdentityAndIncrementReference(ctx context.Context, lbls labels.Labels) (int64, error) {
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
