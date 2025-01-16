package endpoint

import (
	"context"
	"syscall"

	"github.com/microsoft/retina/pkg/common"
	"github.com/pkg/errors"
	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
)

func (w *Watcher) Name() string {
	return watcherName
}

func (w *Watcher) Start(ctx context.Context) error {
	w.l.Info("endpoint watcher started")

	// Create a channel to receive netlink events.
	netlinkEvCh := make(chan netlink.LinkUpdate)
	done := make(chan struct{})
	// Options for subscribing to link updates. We want to list existing links.
	opt := netlink.LinkSubscribeOptions{
		ListExisting: true,
	}
	// Subscribe to link updates.
	if err := netlink.LinkSubscribeWithOptions(netlinkEvCh, done, opt); err != nil {
		return errors.Wrap(err, "failed to subscribe to link updates")
	}
	defer close(done)

	for {
		select {
		case <-ctx.Done():
			w.l.Info("stopping endpoint watcher")
			return nil
		case ev := <-netlinkEvCh:
			// Filter for veth devices.
			if ev.Link.Type() == "veth" {
				veth := ev.Link.(*netlink.Veth)
				switch ev.Header.Type {
				case syscall.RTM_NEWLINK:
					// Check if the veth device is up.
					if veth.Attrs().OperState == netlink.OperUp {
						w.l.Info("veth device is up", zap.String("veth", veth.Attrs().Name))
						w.p.Publish(common.PubSubEndpoints, NewEndpointEvent(EndpointCreated, *veth.Attrs()))
					}
				case syscall.RTM_DELLINK:
					// Check if the veth device is down.
					if veth.Attrs().OperState == netlink.OperDown {
						w.l.Info("veth device is down", zap.String("veth", veth.Attrs().Name))
						w.p.Publish(common.PubSubEndpoints, NewEndpointEvent(EndpointDeleted, *veth.Attrs()))
					}
				}
			}
		}
	}
}

func (w *Watcher) Stop(_ context.Context) error {
	w.l.Info("stopping veth watcher")
	return nil
}
