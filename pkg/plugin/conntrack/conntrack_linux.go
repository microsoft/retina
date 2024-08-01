// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

// package conntrack implements a conntrack plugin for Retina.
package conntrack

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/rlimit"
	"github.com/microsoft/retina/pkg/log"
	plugincommon "github.com/microsoft/retina/pkg/plugin/common"
	"github.com/microsoft/retina/pkg/utils"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go@master -cflags "-g -O2 -Wall -D__TARGET_ARCH_${GOARCH} -Wall" -target ${GOARCH} -type ct_v4_key conntrack ./_cprog/conntrack.c -- -I../lib/_${GOARCH} -I../lib/common/libbpf/_src

var (
	ct   *Conntrack
	once sync.Once
)

// Define TCP flag constants
const (
	TCP_FIN = 0x01
	TCP_SYN = 0x02
	TCP_RST = 0x04
	TCP_PSH = 0x08
	TCP_ACK = 0x10
	TCP_URG = 0x20
	TCP_ECE = 0x40
	TCP_CWR = 0x80
)

// decodeFlags decodes the TCP flags into a human-readable string
func decodeFlags(flags uint8) string {
	var flagDescriptions []string
	if flags&TCP_FIN != 0 {
		flagDescriptions = append(flagDescriptions, "FIN")
	}
	if flags&TCP_SYN != 0 {
		flagDescriptions = append(flagDescriptions, "SYN")
	}
	if flags&TCP_RST != 0 {
		flagDescriptions = append(flagDescriptions, "RST")
	}
	if flags&TCP_PSH != 0 {
		flagDescriptions = append(flagDescriptions, "PSH")
	}
	if flags&TCP_ACK != 0 {
		flagDescriptions = append(flagDescriptions, "ACK")
	}
	if flags&TCP_URG != 0 {
		flagDescriptions = append(flagDescriptions, "URG")
	}
	if flags&TCP_ECE != 0 {
		flagDescriptions = append(flagDescriptions, "ECE")
	}
	if flags&TCP_CWR != 0 {
		flagDescriptions = append(flagDescriptions, "CWR")
	}
	if len(flagDescriptions) == 0 {
		return "None"
	}
	return strings.Join(flagDescriptions, ", ")
}

func decodeProto(proto uint8) string {
	switch proto {
	case 1:
		return "ICMP"
	case 6:
		return "TCP"
	case 17:
		return "UDP"
	default:
		return "Unknown"
	}
}

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
	ct.ctmap = objs.RetinaConntrackMap

	return ct, nil
}

// Close cleans up the Conntrack plugin.
func (ct *Conntrack) Close() {
	if ct.objs != nil {
		ct.objs.Close()
	}
}

// Run starts the Conntrack garbage collection loop.
func (ct *Conntrack) Run(ctx context.Context) error {
	ticker := time.NewTicker(15 * time.Second) //nolint:gomnd // 10 seconds
	defer ticker.Stop()

	ct.l.Info("Starting Conntrack GC loop")

	for {
		select {
		case <-ctx.Done():
			ct.Close()
			return nil
		case <-ticker.C:
			var key conntrackCtV4Key
			var value conntrackCtValue

			var noOfCtEntries, entriesDeleted int
			// List of keys to be deleted
			var keysToDelete []conntrackCtV4Key
			iter := ct.ctmap.Iterate()
			for iter.Next(&key, &value) {
				noOfCtEntries++
				if value.IsClosing == 1 {
					keysToDelete = append(keysToDelete, key)
				}
				// Log the conntrack entry
				srcIP := utils.Int2ip(key.SrcIp).To4()
				dstIP := utils.Int2ip(key.DstIp).To4()
				sourcePortShort := uint32(utils.HostToNetShort(key.SrcPort))
				destinationPortShort := uint32(utils.HostToNetShort(key.DstPort))
				ct.l.Info("Conntrack entry", zap.String("srcIP", srcIP.String()),
					zap.Uint32("srcPort", sourcePortShort),
					zap.String("dstIP", dstIP.String()),
					zap.Uint32("dstPort", destinationPortShort),
					zap.String("proto", decodeProto(key.Proto)),
					zap.Uint32("lifetime", value.Lifetime),
					zap.Uint16("isClosing", value.IsClosing),
					zap.String("flags_seen", decodeFlags(value.FlagsSeen)),
					zap.Uint32("last_reported", value.LastReport),
				)
			}
			if err := iter.Err(); err != nil {
				ct.l.Error("Iterate failed", zap.Error(err))
			}
			// Delete the conntrack entries
			for _, key := range keysToDelete {
				if err := ct.ctmap.Delete(key); err != nil {
					ct.l.Error("Delete failed", zap.Error(err))
				} else {
					entriesDeleted++
				}
			}
			ct.l.Info("Number of conntrack entries", zap.Int("noOfCtEntries", noOfCtEntries), zap.Int("entriesDeleted", entriesDeleted))
		}
	}
}
