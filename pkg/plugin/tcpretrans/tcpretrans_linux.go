// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

// Package tcpretrans contains the Retina tcpretrans plugin. It utilizes eBPF to trace TCP retransmissions.
package tcpretrans

import (
	"context"
	"errors"
	"fmt"
	"net"
	"unsafe"

	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
	"github.com/microsoft/retina/internal/ktime"
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/metrics"
	plugincommon "github.com/microsoft/retina/pkg/plugin/common"
	"github.com/microsoft/retina/pkg/plugin/registry"
	"github.com/microsoft/retina/pkg/utils"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
)

// Per-arch target needed because vmlinux.h differs between amd64/arm64.
// Cross-generate: GOARCH=arm64 go generate ./pkg/plugin/tcpretrans/...
//
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go@v0.18.0 -cflags "-Wall" -target ${GOARCH} -type tcpretrans_event tcpretrans ./_cprog/tcpretrans.c -- -I../lib/_${GOARCH} -I../lib/common/libbpf/_src

const (
	// perCPUBuffer is the starting perf buffer size in pages per CPU.
	// NewPerfReader halves this on ENOMEM down to 1 page. Retransmits are
	// comparatively rare (much less frequent than packet drops), so 16 pages
	// (64 KiB per CPU) is plenty — same starting point as dropreason.
	perCPUBuffer  = 16
	recordsBuffer = 500 // Channel buffer for records
	workers       = 2   // Number of worker goroutines
)

func init() {
	registry.Add(name, New)
}

func New(cfg *kcfg.Config) registry.Plugin {
	return &tcpretrans{
		cfg: cfg,
		l:   log.Logger().Named(name),
	}
}

func (t *tcpretrans) Name() string {
	return name
}

// Generate and Compile are no-ops. The plugin manager lifecycle requires them,
// but tcpretrans uses bpf2go which pre-compiles the BPF program at build time
// and embeds it in the binary — no runtime code generation or compilation needed.
func (t *tcpretrans) Generate(_ context.Context) error { return nil }
func (t *tcpretrans) Compile(_ context.Context) error  { return nil }

func (t *tcpretrans) Init() error {
	if !t.cfg.EnablePodLevel {
		t.l.Warn("tcpretrans will not init because pod level is disabled")
		return nil
	}

	// tcpretrans has a single per-CPU perf event array and no shared maps,
	// so there's nothing to pin — leaving CollectionOptions nil avoids
	// dropping a dangling entry under /sys/fs/bpf.
	objs := &tcpretransObjects{}
	if err := loadTcpretransObjects(objs, nil); err != nil {
		return fmt.Errorf("failed to load eBPF objects: %w", err)
	}
	// Clean up loaded objects if a later step fails.
	ok := false
	defer func() {
		if !ok {
			objs.Close()
		}
	}()

	// Attach to the tcp/tcp_retransmit_skb tracepoint (stable API, kernel 4.15+)
	tp, err := link.Tracepoint("tcp", "tcp_retransmit_skb", objs.RetinaTcpRetransmitSkb, nil)
	if err != nil {
		return fmt.Errorf("failed to attach tracepoint tcp/tcp_retransmit_skb: %w", err)
	}
	defer func() {
		if !ok {
			tp.Close()
		}
	}()

	reader, err := plugincommon.NewPerfReader(t.l, objs.RetinaTcpretransEvents, perCPUBuffer, 1)
	if err != nil {
		return fmt.Errorf("failed to create perf reader: %w", err)
	}

	t.objs = objs
	t.hooks = append(t.hooks, tp)
	t.reader = reader
	ok = true

	t.l.Info("tcpretrans plugin initialized")
	return nil
}

func (t *tcpretrans) Start(ctx context.Context) error {
	if !t.cfg.EnablePodLevel {
		t.l.Warn("tcpretrans will not start because pod level is disabled")
		return nil
	}

	if enricher.IsInitialized() {
		t.enricher = enricher.Instance()
	} else {
		t.l.Warn("retina enricher is not initialized")
	}

	t.recordsChannel = make(chan perf.Record, recordsBuffer)

	return t.run(ctx)
}

func (t *tcpretrans) run(ctx context.Context) error {
	for range workers {
		t.wg.Add(1)
		go t.processRecord(ctx)
	}
	// readEvents is deliberately not tracked in wg: its reader.Read() call
	// blocks until the perf reader is closed, which we do immediately below
	// on ctx cancellation to unblock it.
	go t.readEvents(ctx)

	<-ctx.Done()
	// Close the reader before waiting on workers so readEvents unblocks
	// from its pending Read() instead of racing to send records onto a
	// channel that no worker is draining anymore. Safe to call Close twice
	// — Stop() will no-op if the reader has already been closed.
	if t.reader != nil {
		if err := t.reader.Close(); err != nil {
			t.l.Warn("failed to close perf reader", zap.Error(err))
		}
	}
	t.wg.Wait()
	return nil
}

func (t *tcpretrans) readEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			record, err := t.reader.Read()
			if err != nil {
				if errors.Is(err, perf.ErrClosed) {
					return
				}
				t.l.Error("Error reading perf event", zap.Error(err))
				continue
			}

			if record.LostSamples > 0 {
				metrics.LostEventsCounter.WithLabelValues(utils.Kernel, name).Add(float64(record.LostSamples))
				continue
			}

			select {
			case t.recordsChannel <- record:
			default:
				metrics.LostEventsCounter.WithLabelValues(utils.BufferedChannel, name).Inc()
			}
		}
	}
}

func (t *tcpretrans) processRecord(ctx context.Context) {
	defer t.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case record := <-t.recordsChannel:
			t.handleTCPRetransEvent(record)
		}
	}
}

func (t *tcpretrans) handleTCPRetransEvent(record perf.Record) {
	eventSize := int(unsafe.Sizeof(tcpretransTcpretransEvent{}))
	if len(record.RawSample) < eventSize {
		return
	}

	event := (*tcpretransTcpretransEvent)(unsafe.Pointer(&record.RawSample[0])) //nolint:gosec // perf record is aligned

	var srcIP, dstIP net.IP
	switch event.Af {
	case 4: // IPv4
		var srcBuf, dstBuf [net.IPv4len]byte
		*(*uint32)(unsafe.Pointer(&srcBuf[0])) = event.SrcIp //nolint:gosec // same size
		*(*uint32)(unsafe.Pointer(&dstBuf[0])) = event.DstIp //nolint:gosec // same size
		srcIP = srcBuf[:]
		dstIP = dstBuf[:]
	case 6: // IPv6
		srcIP = event.SrcIp6[:]
		dstIP = event.DstIp6[:]
	default:
		return
	}

	fl := utils.ToFlow(
		t.l,
		ktime.MonotonicOffset.Nanoseconds()+int64(event.Timestamp), //nolint:gosec // timestamp fits in int64
		srcIP, dstIP,
		uint32(event.SrcPort), uint32(event.DstPort),
		unix.IPPROTO_TCP, 0,
		utils.Verdict_RETRANSMISSION,
	)
	if fl == nil {
		return
	}

	syn := flagBit(event.Tcpflags, 0x02)
	ack := flagBit(event.Tcpflags, 0x10)
	fin := flagBit(event.Tcpflags, 0x01)
	rst := flagBit(event.Tcpflags, 0x04)
	psh := flagBit(event.Tcpflags, 0x08)
	urg := flagBit(event.Tcpflags, 0x20)
	ece := flagBit(event.Tcpflags, 0x40)
	cwr := flagBit(event.Tcpflags, 0x80)
	// NS is always 0: tcp_skb_cb->tcp_flags is a single byte that only holds
	// byte 13 of the TCP header (FIN/SYN/RST/PSH/ACK/URG/ECE/CWR). NS lives
	// in byte 12 and isn't carried in the control block. It's also effectively
	// deprecated — RFC 8311 reclassified ECN nonce as historic — so preserving
	// it would require parsing the full TCP header from the skb clone for no
	// observable benefit.
	utils.AddTCPFlags(fl, syn, ack, fin, rst, psh, urg, ece, cwr, 0)

	ev := &v1.Event{
		Event:     fl,
		Timestamp: fl.Time,
	}

	if t.enricher != nil {
		t.enricher.Write(ev)
	}

	if t.externalChannel != nil {
		select {
		case t.externalChannel <- ev:
		default:
			metrics.LostEventsCounter.WithLabelValues(utils.ExternalChannel, name).Inc()
		}
	}
}

func (t *tcpretrans) Stop() error {
	if !t.cfg.EnablePodLevel {
		return nil
	}
	// Always clean up kernel resources regardless of whether Start() was
	// called. Init() loads BPF objects, attaches the tracepoint, and
	// creates a perf reader — all of which must be released.
	if t.reader != nil {
		// Idempotent: run() already closes the reader on normal shutdown,
		// in which case cilium/ebpf returns nil here.
		if err := t.reader.Close(); err != nil {
			t.l.Warn("failed to close perf reader", zap.Error(err))
		}
	}
	for _, h := range t.hooks {
		if err := h.Close(); err != nil {
			t.l.Warn("failed to close hook", zap.Error(err))
		}
	}
	if t.objs != nil {
		if err := t.objs.Close(); err != nil {
			t.l.Warn("failed to close eBPF objects", zap.Error(err))
		}
	}
	return nil
}

func (t *tcpretrans) SetupChannel(ch chan *v1.Event) error {
	t.externalChannel = ch
	return nil
}

func flagBit(flags, bit uint8) uint16 {
	if flags&bit != 0 {
		return 1
	}
	return 0
}
