// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

// package conntrack implements a conntrack plugin for Retina.
package conntrack

import (
	"context"
	"fmt"
	"path"
	"runtime"
	"time"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/rlimit"
	"github.com/microsoft/retina/internal/ktime"
	"github.com/microsoft/retina/pkg/loader"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/metrics"
	plugincommon "github.com/microsoft/retina/pkg/plugin/common"
	_ "github.com/microsoft/retina/pkg/plugin/conntrack/_cprog" // nolint // This is needed so cprog is included when vendoring
	"github.com/microsoft/retina/pkg/utils"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

var conntrackMetricsEnabled = false // conntrack metrics global variable

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

// Build dynamic header path
func BuildDynamicHeaderPath() string {
	// Get absolute path to this file during runtime.
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}
	currDir := path.Dir(filename)
	return fmt.Sprintf("%s/%s/%s", currDir, bpfSourceDir, dynamicHeaderFileName)
}

// Generate dynamic header file for conntrack eBPF program.
func GenerateDynamic(ctx context.Context, dynamicHeaderPath string, conntrackMetrics int) error {
	st := fmt.Sprintf("#define ENABLE_CONNTRACK_METRICS %d\n", conntrackMetrics)
	err := loader.WriteFile(ctx, dynamicHeaderPath, st)
	if err != nil {
		return errors.Wrap(err, "failed to write conntrack dynamic header")
	}
	// set a global variable
	if conntrackMetrics == 1 {
		conntrackMetricsEnabled = true
	}
	return nil
}

// gcDeletion is an entry queued for garbage collection, carrying the reason classified
// while its value was still available (the delete loop only has the key).
type gcDeletion struct {
	key    conntrackCtV4Key
	reason string
}

// gcEntryReason classifies a conntrack entry removed by garbage collection, from its
// protocol and the TCP flags seen across both directions.
func gcEntryReason(proto uint8, value *conntrackCtEntry) string {
	switch proto {
	case 17: //nolint:gomnd // UDP
		return "udp_idle"
	case 6: //nolint:gomnd // TCP
		flags := value.FlagsSeenTxDir | value.FlagsSeenRxDir
		switch {
		case flags&TCP_RST != 0:
			return "tcp_rst"
		case flags&TCP_FIN != 0:
			return "tcp_fin"
		default:
			return "tcp_idle"
		}
	default:
		return "other"
	}
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

			var noOfCtEntries, entriesDeleted int
			// List of entries to be deleted, with the reason captured while the value is available.
			var keysToDelete []gcDeletion

			// metrics counters
			var packetsCountTx, packetsCountRx, totConnections uint32
			var bytesCountTx, bytesCountRx uint64
			// Live unknown-direction population (SYN never observed), e.g. connections
			// established before tracking or recreated after LRU eviction.
			var unknownDirectionEntries uint32
			var unknownDirectionBytes uint64

			iter := ct.ctMap.Iterate()
			for iter.Next(&key, &value) {
				noOfCtEntries++
				// Check if the connection is closing or has expired
				if ktime.MonotonicOffset.Seconds()+float64(value.EvictionTime) < float64((time.Now().Unix())) {
					// Iterating a hash map from which keys are being deleted is not safe.
					// So, we store the keys to be deleted in a list and delete them after the iteration.
					keyCopy := key // Copy the key to avoid using the same key in the next iteration
					keysToDelete = append(keysToDelete, gcDeletion{key: keyCopy, reason: gcEntryReason(key.Proto, &value)})
				}
				// Log the conntrack entry
				srcIP := utils.Int2ip(key.SrcIp).To4()
				dstIP := utils.Int2ip(key.DstIp).To4()
				sourcePortShort := uint32(utils.HostToNetShort(key.SrcPort))
				destinationPortShort := uint32(utils.HostToNetShort(key.DstPort))

				// Add conntrack metrics.
				if conntrackMetricsEnabled {
					// Basic metrics, node-level
					ctMeta := value.ConntrackMetadata
					totConnections++
					bytesCountTx += ctMeta.BytesTxCount
					bytesCountRx += ctMeta.BytesRxCount
					packetsCountTx += ctMeta.PacketsTxCount
					packetsCountRx += ctMeta.PacketsRxCount
					if value.IsDirectionUnknown {
						unknownDirectionEntries++
						unknownDirectionBytes += ctMeta.BytesTxCount + ctMeta.BytesRxCount
					}
				}

				ct.l.Debug("conntrack entry",
					zap.String("src_ip", srcIP.String()),
					zap.Uint32("src_port", sourcePortShort),
					zap.String("dst_ip", dstIP.String()),
					zap.Uint32("dst_port", destinationPortShort),
					zap.String("proto", decodeProto(key.Proto)),
					zap.Uint32("eviction_time", value.EvictionTime),
					zap.Uint8("traffic_direction", value.TrafficDirection),
					zap.String("flags_seen_tx_dir", decodeFlags(value.FlagsSeenTxDir)),
					zap.String("flags_seen_rx_dir", decodeFlags(value.FlagsSeenRxDir)),
					zap.Uint32("last_reported_tx_dir", value.LastReportTxDir),
					zap.Uint32("last_reported_rx_dir", value.LastReportRxDir),
					zap.Bool("is_direction_unknown", value.IsDirectionUnknown),
				)
			}
			if err := iter.Err(); err != nil {
				ct.l.Error("Iterate failed", zap.Error(err))
			}

			// create metrics
			if conntrackMetricsEnabled {
				metrics.ConntrackPacketsTx.WithLabelValues().Set(float64(packetsCountTx))
				metrics.ConntrackBytesTx.WithLabelValues().Set(float64(bytesCountTx))
				metrics.ConntrackPacketsRx.WithLabelValues().Set(float64(packetsCountRx))
				metrics.ConntrackBytesRx.WithLabelValues().Set(float64(bytesCountRx))
				metrics.ConntrackTotalConnections.WithLabelValues().Set(float64(totConnections))
				metrics.ConntrackUnknownDirectionConnections.WithLabelValues().Set(float64(unknownDirectionEntries))
				metrics.ConntrackUnknownDirectionBytes.WithLabelValues().Set(float64(unknownDirectionBytes))
			}

			// Delete the conntrack entries
			for _, d := range keysToDelete {
				if err := ct.ctMap.Delete(d.key); err != nil {
					// Should only happen in a high connection churn scenario, e.g. the entry was
					// already deleted in-kernel on connection teardown between iteration and now.
					ct.l.Debug("Delete failed", zap.Error(err))
				} else {
					entriesDeleted++
					metrics.ConntrackGCEntriesCounter.WithLabelValues(d.reason).Inc()
				}
			}
			ct.l.Debug("conntrack GC completed", zap.Int("number_of_entries", noOfCtEntries), zap.Int("entries_deleted", entriesDeleted))
		}
	}
}
