package resources

import (
	"context"
	"sync"

	cid "github.com/cilium/cilium/pkg/identity"
	cilium_api_v2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	"github.com/cilium/cilium/pkg/k8s/resource"
	"github.com/cilium/hive/cell"
	"github.com/microsoft/retina/pkg/hubble/common"
	"github.com/microsoft/retina/pkg/log"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

var _ common.LabelCache = (*CiliumIdentityHandler)(nil)

const (
	CiliumIdentityHandlerName = "cilium-identity-handler"
)

// CiliumIdentityHandler watches CiliumIdentity resources using resource.Resource
type CiliumIdentityHandler struct {
	identities resource.Resource[*cilium_api_v2.CiliumIdentity]

	// Internal cache for identity management
	mu                 sync.RWMutex
	labelsByIdentityID map[cid.NumericIdentity][]string
	ctxCancelFunc      context.CancelFunc

	logger *log.ZapLogger
}

type CiliumIdentityHandlerParams struct {
	cell.In

	Lifecycle  cell.Lifecycle
	Identities resource.Resource[*cilium_api_v2.CiliumIdentity]
}

type CiliumIdentityHandlerOut struct {
	cell.Out

	common.LabelCache
	*CiliumIdentityHandler
}

// NewCiliumIdentityHandler creates a new CiliumIdentityHandler instance
func NewCiliumIdentityHandler(params CiliumIdentityHandlerParams) CiliumIdentityHandlerOut {
	handler := &CiliumIdentityHandler{
		identities:         params.Identities,
		labelsByIdentityID: make(map[cid.NumericIdentity][]string),
		logger:             log.Logger().Named(CiliumIdentityHandlerName),
	}

	params.Lifecycle.Append(handler)

	return CiliumIdentityHandlerOut{
		LabelCache:            handler,
		CiliumIdentityHandler: handler,
	}
}

func (h *CiliumIdentityHandler) Start(cell.HookContext) error {
	ctx, cancel := context.WithCancel(context.Background())
	h.ctxCancelFunc = cancel
	go h.run(ctx)
	h.logger.Info("CiliumIdentity handler started")
	return nil
}

func (h *CiliumIdentityHandler) Stop(cell.HookContext) error {
	if h.ctxCancelFunc != nil {
		h.ctxCancelFunc()
	}
	h.logger.Info("CiliumIdentity handler stopped")
	return nil
}

func (h *CiliumIdentityHandler) run(ctx context.Context) {
	h.logger.Info("Starting CiliumIdentity event handler")

	identityEvents := h.identities.Events(ctx)

	for {
		select {
		case ev, ok := <-identityEvents:
			if !ok {
				h.logger.Info("CiliumIdentity events channel closed")
				return
			}

			h.handleEvent(ev)
			ev.Done(nil)

		case <-ctx.Done():
			h.logger.Info("CiliumIdentity event handler stopped")
			return
		}
	}
}

func (h *CiliumIdentityHandler) handleEvent(ev resource.Event[*cilium_api_v2.CiliumIdentity]) {
	switch ev.Kind {
	case resource.Sync:
		// Ignore sync events
		h.logger.Debug("CiliumIdentity sync event received", zap.String("key", ev.Key.String()))
	case resource.Upsert:
		if ev.Object != nil {
			h.logger.Debug("CiliumIdentity upsert event", zap.String("key", ev.Key.String()))
			if err := h.updateIdentityCache(ev.Object); err != nil {
				h.logger.Error("Failed to update identity cache", zap.Error(err), zap.String("key", ev.Key.String()))
			}
		}
	case resource.Delete:
		h.logger.Debug("CiliumIdentity delete event", zap.String("key", ev.Key.String()))
		h.removeIdentityFromCache(ev.Key.Name)
	}
}

func (h *CiliumIdentityHandler) GetLabelsFromSecurityIdentity(id cid.NumericIdentity) []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	// Retrieve labels from the cache
	labels, exists := h.labelsByIdentityID[id]
	if !exists {
		h.logger.Debug("Identity not found in cache", zap.Uint32("identity", id.Uint32()))
		return nil
	}
	h.logger.Debug("Retrieved labels from cache", zap.Uint32("identity", id.Uint32()), zap.Strings("labels", labels))
	return labels
}

func (h *CiliumIdentityHandler) updateIdentityCache(identity *cilium_api_v2.CiliumIdentity) error {
	id, err := cid.ParseNumericIdentity(identity.Name)
	if err != nil {
		return errors.Wrapf(err, "invalid identity name %s", identity.Name)
	}
	// Parse the security labels
	secLabels := make([]string, 0, len(identity.SecurityLabels))
	for k, v := range identity.SecurityLabels {
		secLabels = append(secLabels, k+"="+v)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	// Update the cache with the new labels
	h.labelsByIdentityID[id] = secLabels

	return nil
}

func (h *CiliumIdentityHandler) removeIdentityFromCache(name string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	id, err := cid.ParseNumericIdentity(name)
	if err != nil {
		h.logger.Error("Failed to parse identity name", zap.Error(err), zap.String("identity", name))
		return
	}

	if _, exists := h.labelsByIdentityID[id]; exists {
		delete(h.labelsByIdentityID, id)
		h.logger.Debug("Removed identity from cache", zap.String("identity", name))
	} else {
		h.logger.Debug("Identity not found in cache", zap.String("identity", name))
	}
}
