// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

// package conntrack implements a conntrack plugin for Retina.
package conntrack

import (
	"context"
	"time"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/rlimit"
	"github.com/microsoft/retina/pkg/log"
	plugincommon "github.com/microsoft/retina/pkg/plugin/common"
	_ "github.com/microsoft/retina/pkg/plugin/conntrack/_cprog" // nolint // This is needed so cprog is included when vendoring
	"github.com/microsoft/retina/pkg/utils"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go@master -cflags "-g -O2 -Wall -D__TARGET_ARCH_${GOARCH} -Wall" -target ${GOARCH} -type ct_v4_key conntrack ./_cprog/conntrack.c -- -I../lib/_${GOARCH} -I../lib/common/libbpf/_src -I../lib/common/libbpf/_include/linux -I../lib/common/libbpf/_include/uapi/linux -I../lib/common/libbpf/_include/asm

// Init initializes the conntrack eBPF map in the kernel for the first time.
// This function should be called in the init container since
// it requires securityContext.privileged to be true.
func Init() error {
	// Allow the current process to lock memory for eBPF resources.
	if err := rlimit.RemoveMemlock(); err != nil {
		return errors.Wrapf(err, "failed to remove memlock limit")
	}

	objs := &conntrackObjects{}
	err := loadConntrackObjects(objs, &ebpf.CollectionOptions{
		Maps: ebpf.MapOptions{
			PinPath: plugincommon.MapPath,
		},
	})
	if err != nil {
		return errors.Wrap(err, "failed to load conntrack objects")
	}
	return nil
}

// New returns a new Conntrack instance.
func New() (*Conntrack, error) {
	ct := &Conntrack{
		l:           log.Logger().Named("conntrack"),
		gcFrequency: defaultGCFrequency,
	}

	objs := &conntrackObjects{}
	err := loadConntrackObjects(objs, &ebpf.CollectionOptions{
		Maps: ebpf.MapOptions{
			PinPath: plugincommon.MapPath,
		},
	})
	if err != nil {
		ct.l.Error("loadConntrackObjects failed", zap.Error(err))
		return nil, errors.Wrap(err, "failed to load conntrack objects")
	}

	ct.objs = objs
	// Get the conntrack map from the objects
	ct.ctMap = objs.RetinaConntrack
	return ct, nil
}

// Run starts the Conntrack garbage collection loop.
func (ct *Conntrack) Run(ctx context.Context) error {
	ticker := time.NewTicker(ct.gcFrequency)
	defer ticker.Stop()

	ct.l.Info("Starting Conntrack GC loop")

	for {
		select {
		case <-ctx.Done():
			ct.l.Info("Stopping conntrack GC loop")
			if ct.objs != nil {
				err := ct.objs.Close()
				if err != nil {
					ct.l.Error("objs.Close failed", zap.Error(err))
					return errors.Wrap(err, "failed to close conntrack objects")
				}
			}
			return nil
		case <-ticker.C:
			var key conntrackCtV4Key
			var value conntrackCtEntry

			var noOfCtEntries int

			iter := ct.ctMap.Iterate()
			for iter.Next(&key, &value) {
				noOfCtEntries++
				// Log the conntrack entry
				srcIP := utils.Int2ip(key.SrcIp).To4()
				dstIP := utils.Int2ip(key.DstIp).To4()
				sourcePortShort := uint32(utils.HostToNetShort(key.SrcPort))
				destinationPortShort := uint32(utils.HostToNetShort(key.DstPort))
				ct.l.Debug("conntrack entry",
					zap.String("src_ip", srcIP.String()),
					zap.Uint32("src_port", sourcePortShort),
					zap.String("dst_ip", dstIP.String()),
					zap.Uint32("dst_port", destinationPortShort),
					zap.String("proto", decodeProto(key.Proto)),
					zap.Uint32("eviction_time", value.EvictionTime),
					zap.Uint8("traffic_direction", value.TrafficDirection),
					zap.Bool("is_closing", value.IsClosing),
					zap.String("flags_seen_tx_dir", decodeFlags(value.FlagsSeenTxDir)),
					zap.String("flags_seen_rx_dir", decodeFlags(value.FlagsSeenRxDir)),
					zap.Uint32("last_reported_tx_dir", value.LastReportTxDir),
					zap.Uint32("last_reported_rx_dir", value.LastReportRxDir),
				)
			}
			if err := iter.Err(); err != nil {
				ct.l.Error("Iterate failed", zap.Error(err))
			}

			ct.l.Debug("conntrack GC completed", zap.Int("number_of_entries", noOfCtEntries))
		}
	}
}
