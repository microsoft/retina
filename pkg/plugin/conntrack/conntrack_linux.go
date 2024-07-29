// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

// package conntrack implements a conntrack plugin for Retina.
package conntrack

import (
	"context"
	"sync"
	"time"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/rlimit"
	"github.com/microsoft/retina/internal/ktime"
	"github.com/microsoft/retina/pkg/log"
	plugincommon "github.com/microsoft/retina/pkg/plugin/common"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go@master -cc clang-14 -cflags "-g -O2 -Wall -D__TARGET_ARCH_${GOARCH} -Wall" -target ${GOARCH} -type ct_key conntrack ./_cprog/conntrack.c -- -I../lib/_${GOARCH} -I../lib/common/libbpf/_src

var (
	ct   *Conntrack
	once sync.Once
)

type Conntrack struct {
	l     *log.ZapLogger
	objs  *conntrackObjects
	ctmap *ebpf.Map
}

func Init() (*Conntrack, error) {
	once.Do(func() {
		ct = &Conntrack{}
	})
	if ct.l == nil {
		ct.l = log.Logger().Named("conntrack")
	}
	if ct.objs != nil {
		return ct, nil
	}

	// Allow the current process to lock memory for eBPF resources.
	if err := rlimit.RemoveMemlock(); err != nil {
		ct.l.Error("RemoveMemlock failed", zap.Error(err))
		return ct, errors.Wrapf(err, "failed to remove memlock limit")
	}

	objs := &conntrackObjects{}
	err := loadConntrackObjects(objs, &ebpf.CollectionOptions{
		Maps: ebpf.MapOptions{
			PinPath: plugincommon.MapPath,
		},
	})
	if err != nil {
		ct.l.Error("loadConntrackObjects failed", zap.Error(err))
		return ct, err
	}

	ct.objs = objs

	// Get the conntrack map from the objects
	ct.ctmap = objs.conntrackMaps.RetinaConntrackMap

	return ct, nil
}

// Close cleans up the Conntrack plugin.
func (ct *Conntrack) Close() {
	if ct.objs != nil {
		ct.objs.Close()
	}
}

// gc loops through the conntrack map and deletes entries older than the specified timeout.
func (ct *Conntrack) gc(timeout time.Duration) {
	ct.l.Debug("Running Conntrack GC loop")

	var key conntrackCtKey
	var value conntrackCtValue

	var noOfCtEntries, noOfCtEntriesDeleted int

	iter := ct.ctmap.Iterate()
	for iter.Next(&key, &value) {
		ct.l.Debug("ct_key", zap.Uint32("src_ip", key.SrcIp), zap.Uint32("dst_ip", key.DstIp), zap.Uint16("src_port", key.SrcPort), zap.Uint16("dst_port", key.DstPort), zap.Uint8("proto", key.Protocol))
		ct.l.Debug("ct_value", zap.Uint64("timestamp", value.Timestamp), zap.Uint8("is_closed", value.IsClosed))

		// If the entry is marked as isClosed, delete the key and continue
		if value.IsClosed == 1 {
			ct.l.Debug("deleting conntrack entry since it is marked as closed",
				zap.Uint32("src_ip", key.SrcIp),
				zap.Uint32("dst_ip", key.DstIp),
				zap.Uint16("src_port", key.SrcPort),
				zap.Uint16("dst_port", key.DstPort),
				zap.Uint8("proto", key.Protocol),
			)
			err := ct.ctmap.Delete(&key)
			if err != nil {
				ct.l.Error("failed to delete conntrack entry", zap.Error(err))
			}
			noOfCtEntriesDeleted++
			continue
		}

		// If the last seen time is older than the timeout, delete the key
		// Convert the timestamp from nanoseconds to a time.Time value
		lastSeen := ktime.MonotonicOffset.Nanoseconds() + int64(value.Timestamp)

		if time.Since(time.Unix(lastSeen, 0)) > timeout {
			ct.l.Debug("deleting conntrack entry since it is older than the timeout",
				zap.Uint32("src_ip", key.SrcIp), zap.Uint32("dst_ip", key.DstIp),
				zap.Uint16("src_port", key.SrcPort),
				zap.Uint16("dst_port", key.DstPort),
				zap.Uint8("proto", key.Protocol),
			)
			err := ct.ctmap.Delete(&key)
			if err != nil {
				ct.l.Error("failed to delete conntrack entry", zap.Error(err))
			}
			noOfCtEntriesDeleted++
		}
	}
	// Log the number of entries and the number of entries deleted
	ct.l.Info("Conntrack GC loop completed", zap.Int("no_of_ct_entries", noOfCtEntries), zap.Int("no_of_ct_entries_deleted", noOfCtEntriesDeleted))
}

// Start starts the Conntrack GC loop. It runs every 30 seconds and deletes entries older than 5 minutes.
func (ct *Conntrack) Run(ctx context.Context) error {
	ticker := time.NewTicker(30 * time.Second) //nolint:gomnd // 30 seconds
	defer ticker.Stop()

	ct.l.Info("Starting Conntrack GC loop")

	for {
		select {
		case <-ctx.Done():
			ct.Close()
			return nil
		case <-ticker.C:
			ct.gc(5 * time.Minute) //nolint:gomnd // 5 minutes
		}
	}
}
