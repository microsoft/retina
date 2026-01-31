// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

// Package tcpretrans contains the Retina tcpretrans plugin. It utilizes eBPF to trace TCP retransmissions.
package tcpretrans

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"path"
	"runtime"

	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/loader"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/metrics"
	plugincommon "github.com/microsoft/retina/pkg/plugin/common"
	"github.com/microsoft/retina/pkg/plugin/registry"
	_ "github.com/microsoft/retina/pkg/plugin/tcpretrans/_cprog" // nolint
	"github.com/microsoft/retina/pkg/utils"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go@master -cflags "-g -O2 -Wall -D__TARGET_ARCH_${GOARCH} -Wall" -target ${GOARCH} -type tcpretrans_event tcpretrans ./_cprog/tcpretrans.c -- -I../lib/_${GOARCH} -I../lib/common/libbpf/_src

const (
	bpfSourceDir      = "_cprog"
	bpfSourceFileName = "tcpretrans.c"
	bpfObjectFileName = "tcpretrans_bpf.o"
	// Buffer sizes
	perCPUBuffer  = 4096 // Per-CPU buffer pages for perf reader
	recordsBuffer = 500  // Channel buffer for records
	workers       = 2    // Number of worker goroutines
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

// absPath returns the absolute path to the directory where this file resides.
func absPath() (string, error) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("failed to determine current file path")
	}
	dir := path.Dir(filename)
	return dir, nil
}

func (t *tcpretrans) Name() string {
	return name
}

func (t *tcpretrans) Generate(ctx context.Context) error {
	return nil
}

func (t *tcpretrans) Compile(ctx context.Context) error {
	dir, err := absPath()
	if err != nil {
		return err
	}

	arch := runtime.GOARCH

	bpfSourceFile := fmt.Sprintf("%s/%s/%s", dir, bpfSourceDir, bpfSourceFileName)
	bpfOutputFile := fmt.Sprintf("%s/%s", dir, bpfObjectFileName)

	includeDir := fmt.Sprintf("-I%s/../lib/_%s", dir, arch)
	libbpfDir := fmt.Sprintf("-I%s/../lib/common/libbpf/_src", dir)

	targetArch := "-D__TARGET_ARCH_x86"
	if arch == "arm64" {
		targetArch = "-D__TARGET_ARCH_arm64"
	}

	err = loader.CompileEbpf(ctx,
		"-target", "bpf", "-Wall", targetArch, "-g", "-O2", "-c",
		bpfSourceFile, "-o", bpfOutputFile, includeDir, libbpfDir)
	if err != nil {
		return errors.Wrap(err, "error compiling tcpretrans eBPF code")
	}
	t.l.Info("tcpretrans plugin eBPF compiled")
	return nil
}

func (t *tcpretrans) Init() error {
	if !t.cfg.EnablePodLevel {
		t.l.Warn("tcpretrans will not init because pod level is disabled")
		return nil
	}

	dir, err := absPath()
	if err != nil {
		return err
	}

	bpfOutputFile := fmt.Sprintf("%s/%s", dir, bpfObjectFileName)

	spec, err := ebpf.LoadCollectionSpec(bpfOutputFile)
	if err != nil {
		t.l.Error("Error loading collection specs", zap.Error(err))
		return errors.Wrap(err, "failed to load eBPF collection spec")
	}

	objs := &tcpretransObjects{}
	if err := spec.LoadAndAssign(objs, &ebpf.CollectionOptions{
		Maps: ebpf.MapOptions{
			PinPath: plugincommon.MapPath,
		},
	}); err != nil {
		t.l.Error("Error loading eBPF programs", zap.Error(err))
		return errors.Wrap(err, "failed to load and assign eBPF objects")
	}

	t.objs = objs

	// Attach kprobe to tcp_retransmit_skb
	kp, err := link.Kprobe("tcp_retransmit_skb", objs.RetinaTcpRetransmitSkb, nil)
	if err != nil {
		t.l.Error("Error attaching kprobe to tcp_retransmit_skb", zap.Error(err))
		return errors.Wrap(err, "failed to attach kprobe to tcp_retransmit_skb")
	}
	t.hooks = append(t.hooks, kp)

	// Create perf reader for events
	t.reader, err = plugincommon.NewPerfReader(t.l, objs.RetinaTcpretransEvents, perCPUBuffer, 1)
	if err != nil {
		t.l.Error("Error creating perf reader", zap.Error(err))
		return errors.Wrap(err, "failed to create perf reader")
	}

	t.l.Info("tcpretrans plugin initialized")
	return nil
}

func (t *tcpretrans) Start(ctx context.Context) error {
	if !t.cfg.EnablePodLevel {
		t.l.Warn("tcpretrans will not start because pod level is disabled")
		return nil
	}

	t.l.Info("Starting tcpretrans plugin")
	t.isRunning = true

	// Set up enricher
	if enricher.IsInitialized() {
		t.enricher = enricher.Instance()
	} else {
		t.l.Warn("retina enricher is not initialized")
	}

	t.recordsChannel = make(chan perf.Record, recordsBuffer)

	return t.run(ctx)
}

func (t *tcpretrans) run(ctx context.Context) error {
	// Start worker goroutines
	for i := 0; i < workers; i++ {
		t.wg.Add(1)
		go t.processRecord(ctx, i)
	}

	// Read from perf buffer
	go t.readEvents(ctx)

	<-ctx.Done()
	t.wg.Wait()
	t.l.Info("tcpretrans plugin stopped")
	return nil
}

func (t *tcpretrans) readEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			t.l.Info("Context done, stopping tcpretrans event reader")
			return
		default:
			record, err := t.reader.Read()
			if err != nil {
				if errors.Is(err, perf.ErrClosed) {
					t.l.Info("Perf reader closed")
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

func (t *tcpretrans) processRecord(ctx context.Context, id int) {
	t.l.Debug("Starting tcpretrans worker", zap.Int("id", id))
	defer t.wg.Done()

	for {
		select {
		case <-ctx.Done():
			t.l.Info("Context done, stopping tcpretrans worker", zap.Int("id", id))
			return
		case record := <-t.recordsChannel:
			t.handleTCPRetransEvent(record)
		}
	}
}

func (t *tcpretrans) handleTCPRetransEvent(record perf.Record) {
	if len(record.RawSample) < int(binary.Size(tcpretransTcpretransEvent{})) {
		t.l.Debug("Record too small", zap.Int("size", len(record.RawSample)))
		return
	}

	var event tcpretransTcpretransEvent
	reader := bytes.NewReader(record.RawSample)
	if err := binary.Read(reader, binary.LittleEndian, &event); err != nil {
		t.l.Error("Error parsing tcpretrans event", zap.Error(err))
		return
	}

	// Only handle IPv4 for now (matching original behavior)
	if event.Af != 4 {
		return
	}

	// Get IP addresses
	srcIP := int2ip(event.SrcIp)
	dstIP := int2ip(event.DstIp)

	// Build flow
	fl := utils.ToFlow(
		t.l,
		int64(event.Timestamp),
		srcIP,
		dstIP,
		uint32(event.SrcPort),
		uint32(event.DstPort),
		unix.IPPROTO_TCP,
		0, // no direction
		utils.Verdict_RETRANSMISSION,
	)

	if fl == nil {
		t.l.Warn("Could not convert event to flow")
		return
	}

	// Parse TCP flags from the flags byte
	syn := flagBit(event.Tcpflags, 0x02)
	ack := flagBit(event.Tcpflags, 0x10)
	fin := flagBit(event.Tcpflags, 0x01)
	rst := flagBit(event.Tcpflags, 0x04)
	psh := flagBit(event.Tcpflags, 0x08)
	urg := flagBit(event.Tcpflags, 0x20)
	ece := flagBit(event.Tcpflags, 0x40)
	cwr := flagBit(event.Tcpflags, 0x80)
	utils.AddTCPFlags(fl, syn, ack, fin, rst, psh, urg, ece, cwr, 0)

	ev := &v1.Event{
		Event:     fl,
		Timestamp: fl.Time,
	}

	// Write the event to the enricher
	if t.enricher != nil {
		t.enricher.Write(ev)
	}

	// Send to external channel
	if t.externalChannel != nil {
		select {
		case t.externalChannel <- ev:
		default:
			metrics.LostEventsCounter.WithLabelValues(utils.ExternalChannel, name).Inc()
		}
	}
}

func (t *tcpretrans) Stop() error {
	if !t.cfg.EnablePodLevel || !t.isRunning {
		return nil
	}

	if t.reader != nil {
		t.reader.Close()
	}

	for _, h := range t.hooks {
		h.Close()
	}

	if t.objs != nil {
		t.objs.Close()
	}

	t.isRunning = false
	t.l.Info("Stopped tcpretrans plugin")
	return nil
}

func (t *tcpretrans) SetupChannel(ch chan *v1.Event) error {
	t.externalChannel = ch
	return nil
}

// Helper functions

func int2ip(nn uint32) net.IP {
	ip := make(net.IP, 4)
	ip[0] = byte(nn)
	ip[1] = byte(nn >> 8)
	ip[2] = byte(nn >> 16)
	ip[3] = byte(nn >> 24)
	return ip
}

func flagBit(flags uint8, bit uint8) uint16 {
	if flags&bit != 0 {
		return 1
	}
	return 0
}
