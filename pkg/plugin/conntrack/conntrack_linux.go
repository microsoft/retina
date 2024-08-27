// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

// package conntrack implements a conntrack plugin for Retina.
package conntrack

import (
	"context"
	"time"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/rlimit"
	"github.com/microsoft/retina/internal/ktime"
	"github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/log"
	plugincommon "github.com/microsoft/retina/pkg/plugin/common"
	_ "github.com/microsoft/retina/pkg/plugin/conntrack/_cprog" // nolint // This is needed so cprog is included when vendoring
	"github.com/microsoft/retina/pkg/utils"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// New creates a packetparser plugin.
//
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go@master -cflags "-g -O2 -Wall -D__TARGET_ARCH_${GOARCH} -Wall" -target ${GOARCH} -type ct_v4_key conntrack ./_cprog/conntrack.c -- -I../lib/_${GOARCH} -I../lib/common/libbpf/_src -I../lib/common/libbpf/_include/linux -I../lib/common/libbpf/_include/uapi/linux -I../lib/common/libbpf/_include/asm
func New(cfg *config.Config) *Conntrack {
	return &Conntrack{
		l:           log.Logger().Named("conntrack"),
		gcFrequency: defaultGCFrequency,
		cfg:         cfg,
	}
}

// Run starts the Conntrack garbage collection loop.
func (ct *Conntrack) Run(ctx context.Context) error {
	if ct.cfg.DataAggregationLevel == config.Low {
		ct.l.Info("conntrack is disabled in low data aggregation level")
		return nil
	}
	// Allow the current process to lock memory for eBPF resources.
	if err := rlimit.RemoveMemlock(); err != nil {
		ct.l.Error("RemoveMemlock failed", zap.Error(err))
		return errors.Wrapf(err, "failed to remove memlock limit")
	}

	objs := &conntrackObjects{}
	err := loadConntrackObjects(objs, &ebpf.CollectionOptions{
		Maps: ebpf.MapOptions{
			PinPath: plugincommon.MapPath,
		},
	})
	if err != nil {
		ct.l.Error("loadConntrackObjects failed", zap.Error(err))
		return errors.Wrap(err, "failed to load conntrack objects")
	}

	ct.objs = objs

	// Get the conntrack map from the objects
	ct.ctMap = objs.RetinaConntrackMap

	ticker := time.NewTicker(ct.gcFrequency)
	defer ticker.Stop()

	ct.l.Info("starting Conntrack GC loop")

	for {
		select {
		case <-ctx.Done():
			ct.l.Info("stopping conntrack GC loop")
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

			var noOfCtEntries, entriesDeleted int
			// List of keys to be deleted
			var keysToDelete []conntrackCtV4Key
			iter := ct.ctMap.Iterate()
			for iter.Next(&key, &value) {
				noOfCtEntries++
				if value.IsClosing || ktime.MonotonicOffset.Seconds()+float64(value.Lifetime) < float64((time.Now().Unix())) {
					// Iterating a hash map from which keys are being deleted is not safe.
					// So, we store the keys to be deleted in a list and delete them after the iteration.
					keyCopy := key // Copy the key to avoid using the same key in the next iteration
					keysToDelete = append(keysToDelete, keyCopy)
				}
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
					zap.Uint32("lifetime", value.Lifetime),
					zap.Uint32("traffic_direction", value.TrafficDirection),
					zap.Bool("is_closing", value.IsClosing),
					zap.String("flags_seen_forward_dir", decodeFlags(value.FlagsSeenForwardDir)),
					zap.String("flags_seen_reply_dir", decodeFlags(value.FlagsSeenReplyDir)),
					zap.Uint32("last_reported_forward_dir", value.LastReportForwardDir),
					zap.Uint32("last_reported_reply_dir", value.LastReportReplyDir),
				)
			}
			if err := iter.Err(); err != nil {
				ct.l.Error("Iterate failed", zap.Error(err))
			}
			// Delete the conntrack entries
			for _, key := range keysToDelete {
				if err := ct.ctMap.Delete(key); err != nil {
					ct.l.Error("Delete failed", zap.Error(err))
				} else {
					entriesDeleted++
				}
			}
			ct.l.Debug("conntrack GC completed", zap.Int("number_of_entries", noOfCtEntries), zap.Int("entries_deleted", entriesDeleted))
		}
	}
}
