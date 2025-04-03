package k8s

import (
	"github.com/cilium/cilium/pkg/identity"
	"github.com/cilium/cilium/pkg/ipcache"
	"github.com/cilium/cilium/pkg/source"
	"github.com/cilium/hive/cell"
	"github.com/microsoft/retina/pkg/common"
	cc "github.com/microsoft/retina/pkg/controllers/cache"
	"github.com/sirupsen/logrus"
)

type params struct {
	cell.In

	Logger    logrus.FieldLogger
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
	l logrus.FieldLogger
}

func (a *APIServerEventHandler) handleAPIServerEvent(event interface{}) {
	cacheEvent, ok := event.(*cc.CacheEvent)
	if !ok {
		a.l.WithField("Event", event).Warn("Received unknown event type")
		return
	}
	switch cacheEvent.Type { //nolint:exhaustive // the default case adequately handles these
	case cc.EventTypeAddAPIServerIPs:
		apiserverObj, ok := cacheEvent.Obj.(*common.APIServerObject)
		if !ok {
			a.l.WithField("Cache Event", cacheEvent).Warn("Received unknown event type")
			return
		}
		ips := apiserverObj.IPs()
		if len(ips) == 0 {
			a.l.WithField("Cache Event", cacheEvent).Warn("Received empty API server IPs")
			return
		}
		for _, ip := range ips {
			//nolint:staticcheck // TODO(timraymond): unclear how to migrate this
			_, err := a.c.Upsert(ip.String(), nil, 0, nil, ipcache.Identity{ID: identity.ReservedIdentityKubeAPIServer, Source: source.Kubernetes})
			if err != nil {
				a.l.WithError(err).WithFields(logrus.Fields{
					"IP": ips[0].String(),
				}).Error("Failed to add API server IPs to ipcache")
				return
			}
		}
		a.l.WithFields(logrus.Fields{
			"IP": ips[0].String(),
		}).Info("Added API server IPs to ipcache")
	case cc.EventTypeDeleteAPIServerIPs:
		apiserverObj, ok := cacheEvent.Obj.(*common.APIServerObject)
		if !ok {
			a.l.WithField("Cache Event", cacheEvent).Warn("Received unknown event type")
			return
		}
		ips := apiserverObj.IPs()
		if len(ips) == 0 {
			a.l.WithField("Cache Event", cacheEvent).Warn("Received empty API server IPs")
			return
		}
		for _, ip := range ips {
			//nolint:staticcheck // TODO(timraymond): unclear how to migrate this
			a.c.Delete(ip.String(), source.Kubernetes)
		}
		a.l.WithFields(logrus.Fields{
			"IP": ips[0].String(),
		}).Info("Deleted API server IPs from ipcache")
	default:
		a.l.WithFields(logrus.Fields{
			"Cache Event": cacheEvent,
			"Type":        cacheEvent.Type,
		}).Warn("Received unknown cache event")
	}
}
