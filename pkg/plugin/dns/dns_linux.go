// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

// Package dns contains the Retina DNS plugin. It uses eBPF socket filters to capture DNS events.
package dns

import (
	"context"
	"net"
	"syscall"
	"unsafe"

	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/perf"
	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	"github.com/microsoft/retina/internal/ktime"
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/metrics"
	plugincommon "github.com/microsoft/retina/pkg/plugin/common"
	"github.com/microsoft/retina/pkg/plugin/registry"
	"github.com/microsoft/retina/pkg/utils"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
)

// Per-arch target needed because vmlinux.h differs between amd64/arm64.
// Cross-generate: GOARCH=arm64 go generate ./pkg/plugin/dns/...
//
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go@v0.18.0 -cflags "-Wall" -target ${GOARCH} -type dns_event dns ./_cprog/dns.c -- -I../lib/_${GOARCH} -I../lib/common/libbpf/_src

const (
	// perCPUBuffer is the max number of pages passed to NewPerfReader.
	// The reader tries this first, then halves until allocation succeeds.
	perCPUBuffer  = 8192
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

func (d *dns) Name() string {
	return name
}

// Generate and Compile are no-ops. The plugin manager lifecycle requires them,
// but DNS uses bpf2go which pre-compiles the BPF program at build time and
// embeds it in the binary — no runtime code generation or compilation needed.
func (d *dns) Generate(_ context.Context) error { return nil }
func (d *dns) Compile(_ context.Context) error  { return nil }

func (d *dns) Init() error {
	objs := &dnsObjects{}
	if err := loadDnsObjects(objs, &ebpf.CollectionOptions{
		Maps: ebpf.MapOptions{
			PinPath: plugincommon.MapPath,
		},
	}); err != nil {
		return errors.Wrap(err, "failed to load eBPF objects")
	}

	// Bind to all interfaces (ifindex=0). The BPF program dedupes by
	// capturing only PACKET_HOST — see dns.c for the filter logic.
	sock, err := utils.OpenRawSocket(0)
	if err != nil {
		objs.Close()
		return errors.Wrap(err, "failed to open raw socket")
	}

	fd := objs.RetinaDnsFilter.FD()
	if err = syscall.SetsockoptInt(sock, syscall.SOL_SOCKET, unix.SO_ATTACH_BPF, fd); err != nil {
		syscall.Close(sock) //nolint:errcheck // best-effort cleanup
		objs.Close()
		return errors.Wrap(err, "failed to attach BPF to socket")
	}

	reader, err := plugincommon.NewPerfReader(d.l, objs.RetinaDnsEvents, perCPUBuffer, 1)
	if err != nil {
		syscall.Close(sock) //nolint:errcheck // best-effort cleanup
		objs.Close()
		return errors.Wrap(err, "failed to create perf reader")
	}

	// Only assign to struct fields after all setup succeeds,
	// so partial Init failures don't leave dangling resources.
	d.objs = objs
	d.sock = sock
	d.reader = reader

	d.l.Info("DNS plugin initialized")
	return nil
}

func (d *dns) Start(ctx context.Context) error {
	d.isRunning = true
	d.recordsChannel = make(chan perf.Record, recordsBuffer)

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
	for i := range workers {
		d.wg.Add(1)
		go d.processRecord(ctx, i)
	}
	// readEvents is not tracked by wg — it blocks inside reader.Read() which
	// is only unblocked by reader.Close() in Stop(). The lifecycle is:
	// ctx cancel → run() returns → Stop() → reader.Close() → readEvents exits.
	go d.readEvents(ctx)

	<-ctx.Done()
	d.wg.Wait()
	return nil
}

func (d *dns) readEvents(ctx context.Context) {
	for {
		// Note: ctx.Done is only checked between Read() calls. Once blocked
		// inside Read(), only reader.Close() (called from Stop) unblocks it.
		select {
		case <-ctx.Done():
			return
		default:
			record, err := d.reader.Read()
			if err != nil {
				if errors.Is(err, perf.ErrClosed) {
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

func (d *dns) processRecord(ctx context.Context, _ int) {
	defer d.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case record := <-d.recordsChannel:
			d.handleDNSEvent(record)
		}
	}
}

func (d *dns) handleDNSEvent(record perf.Record) {
	eventSize := int(unsafe.Sizeof(dnsDnsEvent{}))
	if len(record.RawSample) < eventSize {
		return
	}

	event := (*dnsDnsEvent)(unsafe.Pointer(&record.RawSample[0])) //nolint:gosec // perf record is aligned

	// Increment basic counter (always, regardless of pod-level)
	if event.Qr == 0 {
		metrics.DNSRequestCounter.WithLabelValues().Inc()
	} else {
		metrics.DNSResponseCounter.WithLabelValues().Inc()
	}

	if !d.cfg.EnablePodLevel {
		return
	}

	// Unlike the old IG tracer which observed both ingress and egress per
	// endpoint, this plugin only sees each packet once — the BPF filter
	// drops PACKET_OUTGOING so only RX-side observations remain (see dns.c).
	// We derive direction from QR: queries are egress, responses are ingress.
	const (
		dirIngress uint8 = 2
		dirEgress  uint8 = 3
	)
	var dir uint8
	if event.Qr == 0 {
		dir = dirEgress
	} else {
		dir = dirIngress
	}

	// IP addresses — copy raw bytes, no endian conversion needed
	var srcIP, dstIP net.IP
	switch event.Af {
	case 4:
		var srcBuf, dstBuf [net.IPv4len]byte
		*(*uint32)(unsafe.Pointer(&srcBuf[0])) = event.SrcIp //nolint:gosec // same size
		*(*uint32)(unsafe.Pointer(&dstBuf[0])) = event.DstIp //nolint:gosec // same size
		srcIP = srcBuf[:]
		dstIP = dstBuf[:]
	case 6:
		srcIP = event.SrcIp6[:]
		dstIP = event.DstIp6[:]
	default:
		return
	}

	// Parse DNS name and response addresses from the packet payload.
	var dnsName string
	var addresses []string
	var qtype layers.DNSType
	if packetData := record.RawSample[eventSize:]; len(packetData) > 0 && int(event.DnsOff) < len(packetData) {
		dnsName, addresses, qtype = d.parseDNSPayload(packetData[event.DnsOff:], event.Qr == 1)
	}
	// Prefer BPF-extracted qtype; fall back to gopacket if BPF couldn't
	// parse it (e.g. truncated name, packet too short).
	if event.Qtype != 0 {
		qtype = layers.DNSType(event.Qtype)
	}
	qTypes := []string{qtype.String()}

	var qrStr string
	if event.Qr == 0 {
		qrStr = "Q"
	} else {
		qrStr = "R"
	}

	fl := utils.ToFlow(
		d.l,
		ktime.MonotonicOffset.Nanoseconds()+int64(event.Timestamp), //nolint:gosec // timestamp fits in int64
		srcIP, dstIP,
		uint32(event.SrcPort), uint32(event.DstPort),
		event.Proto, dir,
		utils.Verdict_DNS,
	)
	if fl == nil {
		return
	}

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

	if d.externalChannel != nil {
		select {
		case d.externalChannel <- ev:
		default:
			metrics.LostEventsCounter.WithLabelValues(utils.ExternalChannel, name).Inc()
		}
	}
}

// parseDNSPayload extracts the query name, response addresses, and query type
// from the raw DNS payload using gopacket.
//nolint:nonamedreturns // named returns used by defer recovery
func (d *dns) parseDNSPayload(payload []byte, isResponse bool) (
	dnsName string, addresses []string, qtype layers.DNSType,
) {
	if len(payload) < 12 {
		return "", nil, 0
	}

	// gopacket's DNS decoder can panic on malformed input.
	defer func() {
		if r := recover(); r != nil {
			d.l.Debug("DNS decode panic (malformed packet)", zap.Any("recover", r))
			dnsName, addresses, qtype = "", nil, 0
		}
	}()

	var parser layers.DNS
	if err := parser.DecodeFromBytes(payload, gopacket.NilDecodeFeedback); err != nil {
		return "", nil, 0
	}

	if len(parser.Questions) > 0 {
		dnsName = string(parser.Questions[0].Name) + "."
		qtype = parser.Questions[0].Type
	}

	if isResponse {
		for i := range parser.Answers {
			if parser.Answers[i].IP != nil {
				addresses = append(addresses, parser.Answers[i].IP.String())
			}
		}
	}

	return dnsName, addresses, qtype
}

func (d *dns) Stop() error {
	if !d.isRunning {
		return nil
	}
	if d.reader != nil {
		d.reader.Close()
	}
	if d.recordsChannel != nil {
		close(d.recordsChannel)
	}
	if d.sock != 0 {
		syscall.Close(d.sock) //nolint:errcheck // best-effort cleanup
	}
	if d.objs != nil {
		d.objs.Close()
	}
	d.isRunning = false
	return nil
}

func (d *dns) SetupChannel(c chan *v1.Event) error {
	d.externalChannel = c
	return nil
}
