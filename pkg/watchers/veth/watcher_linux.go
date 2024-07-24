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
			w.l.Info("veth device found", zap.String("device", link.Attrs().Name))
			w.p.Publish(common.PubSubEndpoints, endpoint.NewEndpointEvent(endpoint.EndpointCreated, *link.Attrs()))
		}
	}

	// Create a channel to receive netlink events.
	netlinkEvCh := make(chan netlink.LinkUpdate)
	done := make(chan struct{})
	// Subscribe to link (network interface) updates.
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
			w.l.Info("received netlink event", zap.Any("device", linkAttrsToMap(ev.Link.Attrs())))
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

func (w *Watcher) Stop(_ context.Context) error {
	w.l.Info("stopping veth watcher")
	return nil
}

func linkAttrsToMap(attrs *netlink.LinkAttrs) map[string]any {
	return map[string]any{
		"Name":           attrs.Name,
		"HardwareAddr":   attrs.HardwareAddr.String(),
		"MTU":            attrs.MTU,
		"Flags":          attrs.Flags.String(),
		"RawFlags":       attrs.RawFlags,
		"ParentIndex":    attrs.ParentIndex,
		"MasterIndex":    attrs.MasterIndex,
		"Namespace":      attrs.Namespace,
		"Alias":          attrs.Alias,
		"AltNames":       attrs.AltNames,
		"Statistics":     attrs.Statistics,
		"Promisc":        attrs.Promisc,
		"Allmulti":       attrs.Allmulti,
		"Multi":          attrs.Multi,
		"Xdp":            attrs.Xdp,
		"EncapType":      attrs.EncapType,
		"Protinfo":       attrs.Protinfo,
		"OperState":      attrs.OperState.String(),
		"PhysSwitchID":   attrs.PhysSwitchID,
		"NetNsID":        attrs.NetNsID,
		"NumTxQueues":    attrs.NumTxQueues,
		"NumRxQueues":    attrs.NumRxQueues,
		"TSOMaxSegs":     attrs.TSOMaxSegs,
		"TSOMaxSize":     attrs.TSOMaxSize,
		"GSOMaxSegs":     attrs.GSOMaxSegs,
		"GSOMaxSize":     attrs.GSOMaxSize,
		"GROMaxSize":     attrs.GROMaxSize,
		"GSOIPv4MaxSize": attrs.GSOIPv4MaxSize,
		"GROIPv4MaxSize": attrs.GROIPv4MaxSize,
		"Vfs":            attrs.Vfs,
		"Group":          attrs.Group,
		"PermHWAddr":     attrs.PermHWAddr.String(),
	}
}
