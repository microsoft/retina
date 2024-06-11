package k8s

import (
	"github.com/cilium/cilium/pkg/hive/cell"
	"github.com/cilium/cilium/pkg/identity"
	"github.com/cilium/cilium/pkg/ipcache"
	"github.com/cilium/cilium/pkg/source"
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

func newApiServerEventHandler(p params) *ApiServerEventHandler {
	a := &ApiServerEventHandler{
		c: p.IPCache,
		l: p.Logger,
	}
	return a
}

type ApiServerEventHandler struct {
	c *ipcache.IPCache
	l logrus.FieldLogger
}

func (a *ApiServerEventHandler) handleApiServerEvent(event interface{}) {
	cacheEvent, ok := event.(*cc.CacheEvent)
	if !ok {
		a.l.WithField("Event", event).Warn("Received unknown event type")
		return
	}
	switch cacheEvent.Type {
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
