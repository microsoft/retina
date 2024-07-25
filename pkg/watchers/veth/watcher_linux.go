package veth

import (
	"context"
	"syscall"

	"github.com/microsoft/retina/pkg/common"
	"github.com/microsoft/retina/pkg/watchers/endpoint"
	"github.com/pkg/errors"
	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
)

func (w *Watcher) Name() string {
	return watcherName
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
			w.l.Info("exisiting veth device found", zap.String("name", link.Attrs().Name))
			w.p.Publish(common.PubSubEndpoints, endpoint.NewEndpointEvent(endpoint.EndpointCreated, *link.Attrs()))
		}
	}

	// Create a channel to receive netlink events.
	netlinkEvCh := make(chan netlink.LinkUpdate)
	done := make(chan struct{})
	// Subscribe to link updates.
	if err := netlink.LinkSubscribe(netlinkEvCh, done); err != nil {
		return errors.Wrap(err, "failed to subscribe to link updates")
	}
	defer close(done)

	for {
		select {
		case <-ctx.Done():
			w.l.Info("stopping veth watcher")
			return nil
		case ev := <-netlinkEvCh:
			// Filter for veth devices.
			if ev.Link.Type() == "veth" {
				veth := ev.Link.(*netlink.Veth)
				switch ev.Header.Type {
				case syscall.RTM_NEWLINK:
					// Check if the veth device is up.
					if veth.Attrs().OperState == netlink.OperUp {
						w.l.Info("new veth device added", zap.String("name", veth.Name), zap.String("peer", veth.PeerName), zap.String("mac", veth.HardwareAddr.String()))
						w.p.Publish(common.PubSubEndpoints, endpoint.NewEndpointEvent(endpoint.EndpointCreated, *veth.Attrs()))
					}
				case syscall.RTM_DELLINK:
					// Check if the veth device is down.
					if veth.Attrs().OperState == netlink.OperDown {
						w.l.Info("veth device deleted", zap.String("name", veth.Name), zap.String("peer", veth.PeerName), zap.String("mac", veth.HardwareAddr.String()))
						w.p.Publish(common.PubSubEndpoints, endpoint.NewEndpointEvent(endpoint.EndpointDeleted, *veth.Attrs()))
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
