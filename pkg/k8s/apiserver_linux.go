package k8s

import (
	"log/slog"

	"github.com/cilium/cilium/pkg/identity"
	"github.com/cilium/cilium/pkg/ipcache"
	"github.com/cilium/cilium/pkg/source"
	"github.com/cilium/hive/cell"
	"github.com/microsoft/retina/pkg/common"
	cc "github.com/microsoft/retina/pkg/controllers/cache"
)

type params struct {
	cell.In

	Logger    *slog.Logger
	IPCache   *ipcache.IPCache
	Lifecycle cell.Lifecycle
}

func newAPIServerEventHandler(p params) *APIServerEventHandler {
	a := &APIServerEventHandler{
		c: p.IPCache,
		l: p.Logger,
	}
	return a
}

type APIServerEventHandler struct {
	c *ipcache.IPCache
	l *slog.Logger
}

func (a *APIServerEventHandler) handleAPIServerEvent(event interface{}) {
	cacheEvent, ok := event.(*cc.CacheEvent)
	if !ok {
		a.l.Warn("Received unknown event type", "event", event)
		return
	}
	switch cacheEvent.Type { //nolint:exhaustive // the default case adequately handles these
	case cc.EventTypeAddAPIServerIPs:
		apiserverObj, ok := cacheEvent.Obj.(*common.APIServerObject)
		if !ok {
			a.l.Warn("Received unknown event type", "cacheEvent", cacheEvent)
			return
		}
		ips := apiserverObj.IPs()
		if len(ips) == 0 {
			a.l.Warn("Received empty API server IPs", "cacheEvent", cacheEvent)
			return
		}
		for _, ip := range ips {
			//nolint:staticcheck // TODO(timraymond): unclear how to migrate this
			_, err := a.c.Upsert(ip.String(), nil, 0, nil, ipcache.Identity{ID: identity.ReservedIdentityKubeAPIServer, Source: source.Kubernetes})
			if err != nil {
				a.l.Error("Failed to add API server IPs to ipcache", "error", err, "ip", ip.String())
				return
			}
		}
		a.l.Info("Added API server IPs to ipcache", "ips", ips)
	case cc.EventTypeDeleteAPIServerIPs:
		apiserverObj, ok := cacheEvent.Obj.(*common.APIServerObject)
		if !ok {
			a.l.Warn("Received unknown event type", "cacheEvent", cacheEvent)
			return
		}
		ips := apiserverObj.IPs()
		if len(ips) == 0 {
			a.l.Warn("Received empty API server IPs", "cacheEvent", cacheEvent)
			return
		}
		for _, ip := range ips {
			//nolint:staticcheck // TODO(timraymond): unclear how to migrate this
			a.c.Delete(ip.String(), source.Kubernetes)
		}
		a.l.Info("Deleted API server IPs from ipcache", "ips", ips)
	default:
		a.l.Warn("Received unknown cache event", "cacheEvent", cacheEvent, "type", cacheEvent.Type)
	}
}
