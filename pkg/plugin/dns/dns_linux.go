// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

// Package dns contains the Retina DNS plugin. It uses eBPF socket filters to capture DNS events.
package dns

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"path"
	"runtime"
	"syscall"

	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/perf"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/loader"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/metrics"
	plugincommon "github.com/microsoft/retina/pkg/plugin/common"
	"github.com/microsoft/retina/pkg/plugin/registry"
	"github.com/microsoft/retina/pkg/utils"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go@master -cflags "-g -O2 -Wall -D__TARGET_ARCH_${GOARCH} -Wall" -target ${GOARCH} -type dns_event dns ./_cprog/dns.c -- -I../lib/_${GOARCH} -I../lib/common/libbpf/_src

const (
	bpfSourceDir      = "_cprog"
	bpfSourceFileName = "dns.c"
	bpfObjectFileName = "dns_bpf.o"
	// Socket attach option
	soAttachBPF = 50
	// Buffer sizes
	perCPUBuffer  = 8192 // Per-CPU buffer pages for perf reader
	recordsBuffer = 1000 // Channel buffer for records
	workers       = 2    // Number of worker goroutines
)

func init() {
	registry.Add(name, New)
}

func New(cfg *kcfg.Config) registry.Plugin {
	return &dns{
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

func (d *dns) Name() string {
	return name
}

func (d *dns) Generate(ctx context.Context) error {
	// No dynamic header generation needed for DNS plugin
	d.l.Info("DNS plugin header generation complete")
	return nil
}

func (d *dns) Compile(ctx context.Context) error {
	// Get the absolute path to this file during runtime.
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
		return errors.Wrap(err, "error compiling DNS eBPF code")
	}
	d.l.Info("DNS plugin eBPF compiled")
	return nil
}

func (d *dns) Init() error {
	// Get the absolute path to this file during runtime.
	dir, err := absPath()
	if err != nil {
		return err
	}

	bpfOutputFile := fmt.Sprintf("%s/%s", dir, bpfObjectFileName)

	spec, err := ebpf.LoadCollectionSpec(bpfOutputFile)
	if err != nil {
		d.l.Error("Error loading collection specs", zap.Error(err))
		return errors.Wrap(err, "failed to load eBPF collection spec")
	}

	objs := &dnsObjects{}
	if loadErr := spec.LoadAndAssign(objs, &ebpf.CollectionOptions{
		Maps: ebpf.MapOptions{
			PinPath: plugincommon.MapPath,
		},
	}); loadErr != nil {
		d.l.Error("Error loading eBPF programs", zap.Error(loadErr))
		return errors.Wrap(loadErr, "failed to load and assign eBPF objects")
	}

	d.objs = objs

	// Open raw socket on all interfaces (index 0)
	d.sock, err = utils.OpenRawSocket(0)
	if err != nil {
		d.l.Error("Error opening raw socket", zap.Error(err))
		return errors.Wrap(err, "failed to open raw socket")
	}

	// Attach BPF program to socket
	attachErr := syscall.SetsockoptInt(d.sock, syscall.SOL_SOCKET, soAttachBPF, objs.RetinaDnsFilter.FD())
	if attachErr != nil {
		d.l.Error("Error attaching DNS socket filter", zap.Error(attachErr))
		return errors.Wrap(attachErr, "failed to attach BPF to socket")
	}

	// Create perf reader for events
	d.reader, err = plugincommon.NewPerfReader(d.l, objs.RetinaDnsEvents, perCPUBuffer, 1)
	if err != nil {
		d.l.Error("Error creating perf reader", zap.Error(err))
		return errors.Wrap(err, "failed to create perf reader")
	}

	d.l.Info("DNS plugin initialized")
	return nil
}

func (d *dns) Start(ctx context.Context) error {
	d.l.Info("Starting DNS plugin")
	d.isRunning = true

	d.recordsChannel = make(chan perf.Record, recordsBuffer)

	// Setup enricher if pod-level metrics enabled
	if d.cfg.EnablePodLevel {
		if enricher.IsInitialized() {
			d.enricher = enricher.Instance()
		} else {
			d.l.Warn("retina enricher is not initialized")
		}
	}

	return d.run(ctx)
}

func (d *dns) run(ctx context.Context) error {
	// Start worker goroutines
	for i := 0; i < workers; i++ {
		d.wg.Add(1)
		go d.processRecord(ctx, i)
	}

	// Read from perf buffer
	go d.readEvents(ctx)

	<-ctx.Done()
	d.wg.Wait()
	d.l.Info("DNS plugin stopped")
	return nil
}

func (d *dns) readEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			d.l.Info("Context done, stopping DNS event reader")
			return
		default:
			record, err := d.reader.Read()
			if err != nil {
				if errors.Is(err, perf.ErrClosed) {
					d.l.Info("Perf reader closed")
					return
				}
				d.l.Error("Error reading perf event", zap.Error(err))
				continue
			}

			if record.LostSamples > 0 {
				metrics.LostEventsCounter.WithLabelValues(utils.Kernel, name).Add(float64(record.LostSamples))
				continue
			}

			select {
			case d.recordsChannel <- record:
			default:
				metrics.LostEventsCounter.WithLabelValues(utils.BufferedChannel, name).Inc()
			}
		}
	}
}

func (d *dns) processRecord(ctx context.Context, id int) {
	d.l.Debug("Starting DNS worker", zap.Int("id", id))
	defer d.wg.Done()

	for {
		select {
		case <-ctx.Done():
			d.l.Info("Context done, stopping DNS worker", zap.Int("id", id))
			return
		case record := <-d.recordsChannel:
			d.handleDNSEvent(record)
		}
	}
}

func (d *dns) handleDNSEvent(record perf.Record) {
	// The record contains the dns_event struct followed by the raw packet data
	eventSize := int(binary.Size(dnsDnsEvent{}))
	if len(record.RawSample) < eventSize {
		d.l.Debug("Record too small", zap.Int("size", len(record.RawSample)))
		return
	}

	var event dnsDnsEvent
	reader := bytes.NewReader(record.RawSample)
	if err := binary.Read(reader, binary.LittleEndian, &event); err != nil {
		d.l.Error("Error parsing DNS event", zap.Error(err))
		return
	}

	// Determine if query or response
	var m metrics.CounterVec
	var qrStr string
	if event.Qr == 0 {
		m = metrics.DNSRequestCounter
		qrStr = "Q"
	} else {
		m = metrics.DNSResponseCounter
		qrStr = "R"
	}
	m.WithLabelValues().Inc()

	if !d.cfg.EnablePodLevel {
		return
	}

	// Determine direction from packet type
	var dir uint8
	switch event.PktType {
	case 0: // HOST - incoming
		dir = 2 // Ingress
	case 4: // OUTGOING
		dir = 3 // Egress
	default:
		return
	}

	// Get IP addresses
	var srcIP, dstIP net.IP
	switch event.Af {
	case 4:
		srcIP = int2ip(event.SrcIp)
		dstIP = int2ip(event.DstIp)
	case 6:
		srcIP = event.SrcIp6[:]
		dstIP = event.DstIp6[:]
	default:
		return
	}

	// Parse DNS packet data to extract query name, query types, and response addresses
	var dnsName string
	var qTypes []string
	var addresses []string

	// The packet data follows the event struct
	// DnsOff is the offset from the start of the packet (including ethernet header)
	packetData := record.RawSample[eventSize:]
	if len(packetData) > 0 && int(event.DnsOff) < len(packetData) {
		dnsPayload := packetData[event.DnsOff:]
		dnsName, qTypes, addresses = d.parseDNSPayload(dnsPayload, event.Qr == 1)
	}

	// Build flow
	fl := utils.ToFlow(
		d.l,
		int64(event.Timestamp),
		srcIP,
		dstIP,
		uint32(event.SrcPort),
		uint32(event.DstPort),
		event.Proto,
		dir,
		utils.Verdict_DNS,
	)

	if fl == nil {
		return
	}

	// Add DNS metadata
	ext := utils.NewExtensions()
	utils.AddDNSInfo(fl, ext, qrStr, uint32(event.Rcode), dnsName, qTypes, int(event.Ancount), addresses)
	utils.SetExtensions(fl, ext)

	ev := &v1.Event{
		Event:     fl,
		Timestamp: fl.GetTime(),
	}

	if d.enricher != nil {
		d.enricher.Write(ev)
	}

	// Send to external channel
	if d.externalChannel != nil {
		select {
		case d.externalChannel <- ev:
		default:
			metrics.LostEventsCounter.WithLabelValues(utils.ExternalChannel, name).Inc()
		}
	}
}

// parseDNSPayload parses the DNS payload and extracts the query name, query types,
// and response addresses (for responses).
func (d *dns) parseDNSPayload(payload []byte, isResponse bool) (dnsName string, qTypes, addresses []string) {
	if len(payload) < 12 { // Minimum DNS header size
		return "", nil, nil
	}

	// Use gopacket to parse the DNS layer
	dns := &layers.DNS{}
	if err := dns.DecodeFromBytes(payload, gopacket.NilDecodeFeedback); err != nil {
		d.l.Debug("Failed to parse DNS payload", zap.Error(err))
		return "", nil, nil
	}

	// Extract query name and types from questions
	if len(dns.Questions) > 0 {
		dnsName = string(dns.Questions[0].Name)
		for _, q := range dns.Questions {
			qTypes = append(qTypes, q.Type.String())
		}
	}

	// Extract addresses from answers (only for responses)
	if isResponse {
		for i := range dns.Answers {
			switch dns.Answers[i].Type { //nolint:exhaustive // only handle common DNS types
			case layers.DNSTypeA:
				if dns.Answers[i].IP != nil {
					addresses = append(addresses, dns.Answers[i].IP.String())
				}
			case layers.DNSTypeAAAA:
				if dns.Answers[i].IP != nil {
					addresses = append(addresses, dns.Answers[i].IP.String())
				}
			case layers.DNSTypeCNAME:
				// Include CNAMEs as well for completeness
				if len(dns.Answers[i].CNAME) > 0 {
					addresses = append(addresses, "CNAME:"+string(dns.Answers[i].CNAME))
				}
			}
		}
	}

	return dnsName, qTypes, addresses
}

func (d *dns) Stop() error {
	if !d.isRunning {
		return nil
	}

	if d.reader != nil {
		d.reader.Close()
	}
	if d.sock != 0 {
		syscall.Close(d.sock)
	}
	if d.objs != nil {
		d.objs.Close()
	}

	d.isRunning = false
	d.l.Info("DNS plugin stopped")
	return nil
}

func (d *dns) SetupChannel(c chan *v1.Event) error {
	d.externalChannel = c
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
