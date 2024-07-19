package veth

import (
	"context"
	"syscall"

	"github.com/microsoft/retina/pkg/common"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/pubsub"
	"github.com/microsoft/retina/pkg/watchers/endpoint"
	"github.com/pkg/errors"
	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
)

type Watcher struct {
	l *log.ZapLogger
	p pubsub.PubSubInterface
}

var watcher *Watcher

func NewWatcher() *Watcher {
	if watcher == nil {
		watcher = &Watcher{
			l: log.Logger().Named("veth-watcher"),
			p: pubsub.New(),
		}
	}
	return watcher
}

func (w *Watcher) Start(ctx context.Context) error {
	w.l.Info("starting veth watcher")

	// Get all the existing veth devices.
	// Similar to ip link show type veth
	links, err := netlink.LinkList()
	if err != nil {
		return errors.Wrap(err, "failed to list links")
	}
	for _, link := range links {
		if link.Type() == "veth" {
			w.l.Info("veth device found", zap.String("device", link.Attrs().Name))
			w.p.Publish(common.PubSubEndpoints, endpoint.NewEndpointEvent(endpoint.EndpointCreated, *link.Attrs()))
		}
	}

	// Create a channel to receive netlink events.
	netlinkEv := make(chan netlink.LinkUpdate)
	done := make(chan struct{})
	// Subscribe to link (network interface) updates.
	if err := netlink.LinkSubscribe(netlinkEv, done); err != nil {
		return errors.Wrap(err, "failed to subscribe to link updates")
	}
	defer close(done)

	for {
		select {
		case <-ctx.Done():
			w.l.Info("stopping veth watcher")
			return nil
		case ev := <-netlinkEv:
			// Filter for veth devices.
			w.l.Info("received netlink event", zap.Any("device", ev.Link.Attrs()))
			if ev.Link.Attrs().EncapType == "ether" && ev.Link.Type() == "veth" {
				switch ev.Header.Type {
				case syscall.RTM_NEWLINK:
					w.l.Info("veth device added", zap.String("device", ev.Link.Attrs().Name))
					w.p.Publish(common.PubSubEndpoints, endpoint.NewEndpointEvent(endpoint.EndpointCreated, *ev.Link.Attrs()))
				case syscall.RTM_DELLINK:
					w.l.Info("veth device deleted", zap.String("device", ev.Link.Attrs().Name))
					w.p.Publish(common.PubSubEndpoints, endpoint.NewEndpointEvent(endpoint.EndpointDeleted, *ev.Link.Attrs()))
				}
			}
		}
	}
}
