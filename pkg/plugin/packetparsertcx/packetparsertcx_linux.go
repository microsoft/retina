// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

// package packetparsertcx contains the TCX variant of the Retina packetparser plugin.
// It uses the TCX (TC eXpress) attachment mechanism available in kernel 6.6+.
package packetparsertcx

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"path"
	"runtime"
	"sync"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/cilium/cilium/api/v1/flow"
	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/asm"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
	"github.com/microsoft/retina/internal/ktime"
	"github.com/microsoft/retina/pkg/common"
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/loader"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/metrics"
	plugincommon "github.com/microsoft/retina/pkg/plugin/common"
	"github.com/microsoft/retina/pkg/plugin/conntrack"
	_ "github.com/microsoft/retina/pkg/plugin/lib/_amd64"                            // nolint
	_ "github.com/microsoft/retina/pkg/plugin/lib/_arm64"                            // nolint
	_ "github.com/microsoft/retina/pkg/plugin/lib/common/libbpf/_include/asm"        // nolint
	_ "github.com/microsoft/retina/pkg/plugin/lib/common/libbpf/_include/linux"      // nolint
	_ "github.com/microsoft/retina/pkg/plugin/lib/common/libbpf/_include/uapi/linux" // nolint
	_ "github.com/microsoft/retina/pkg/plugin/lib/common/libbpf/_src"                // nolint
	_ "github.com/microsoft/retina/pkg/plugin/packetparser/_cprog"                   // nolint (shared C headers)
	_ "github.com/microsoft/retina/pkg/plugin/packetparsertcx/_cprog"                // nolint
	"github.com/microsoft/retina/pkg/plugin/registry"
	"github.com/microsoft/retina/pkg/pubsub"
	"github.com/microsoft/retina/pkg/utils"
	"github.com/microsoft/retina/pkg/watchers/endpoint"
	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go@master -cflags "-g -O2 -Wall -D__TARGET_ARCH_${GOARCH} -Wall" -target ${GOARCH} -type packet packetparsertcx ./_cprog/packetparser_tcx.c -- -I../packetparser/_cprog -I../lib/_${GOARCH} -I../lib/common/libbpf/_src -I../lib/common/libbpf/_include/linux -I../lib/common/libbpf/_include/uapi/linux -I../lib/common/libbpf/_include/asm -I../filter/_cprog/ -I../conntrack/_cprog/

var errNoOutgoingLinks = errors.New("could not determine any outgoing links")

func init() {
	registry.Add(name, New)
}

// New creates a packetparsertcx plugin.
func New(cfg *kcfg.Config) registry.Plugin {
	return &packetParserTCX{
		cfg: cfg,
		l:   log.Logger().Named(name),
	}
}

func (p *packetParserTCX) Name() string {
	return name
}

func generateDynamicHeaderPath() (string, error) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("unable to get absolute path to this file")
	}
	dir := path.Dir(filename)
	return fmt.Sprintf("%s/%s/%s", dir, bpfSourceDir, dynamicHeaderFileName), nil
}

func (p *packetParserTCX) Generate(ctx context.Context) error {
	var st string

	dynamicHeaderPath, err := generateDynamicHeaderPath()
	if err != nil {
		return err
	}

	bypassLookupIPOfInterest := 0
	if p.cfg.BypassLookupIPOfInterest {
		p.l.Info("bypassing lookup IP of interest")
		bypassLookupIPOfInterest = 1
	}
	st = fmt.Sprintf("#define BYPASS_LOOKUP_IP_OF_INTEREST %d\n", bypassLookupIPOfInterest)

	conntrackMetrics := 0
	if p.cfg.EnableConntrackMetrics {
		p.l.Info("conntrack metrics enabled")
		conntrackMetrics = 1

		ctDynamicHeaderPath := conntrack.BuildDynamicHeaderPath()
		err = conntrack.GenerateDynamic(ctx, ctDynamicHeaderPath, conntrackMetrics)
		if err != nil {
			return errors.Wrap(err, "failed to generate dynamic header for conntrack")
		}

		st += fmt.Sprintf("#define ENABLE_CONNTRACK_METRICS %d\n", conntrackMetrics)
	}

	p.l.Info("data aggregation level", zap.String("level", p.cfg.DataAggregationLevel.String()))
	st += fmt.Sprintf("#define DATA_AGGREGATION_LEVEL %d\n", p.cfg.DataAggregationLevel)

	p.l.Info("sampling rate", zap.Uint32("rate", p.cfg.DataSamplingRate))
	st += fmt.Sprintf("#define DATA_SAMPLING_RATE %d\n", p.cfg.DataSamplingRate)

	err = loader.WriteFile(ctx, dynamicHeaderPath, st)
	if err != nil {
		return errors.Wrap(err, "failed to write dynamic header")
	}
	p.l.Info("PacketParserTCX header generated at", zap.String("path", dynamicHeaderPath))
	return nil
}

func (p *packetParserTCX) Compile(ctx context.Context) error {
	dir, err := absPath()
	if err != nil {
		return err
	}

	bpfSourceFile := fmt.Sprintf("%s/%s/%s", dir, bpfSourceDir, bpfSourceFileName)
	bpfOutputFile := fmt.Sprintf("%s/%s", dir, bpfObjectFileName)
	arch := runtime.GOARCH
	archLibDir := fmt.Sprintf("-I%s/../lib/_%s", dir, arch)
	filterDir := fmt.Sprintf("-I%s/../filter/_cprog/", dir)
	conntrackDir := fmt.Sprintf("-I%s/../conntrack/_cprog/", dir)
	// Include packetparser _cprog for shared headers (packetparser.h, packetparse.h)
	packetparserDir := fmt.Sprintf("-I%s/../packetparser/_cprog/", dir)
	libbpfSrcDir := fmt.Sprintf("-I%s/../lib/common/libbpf/_src", dir)
	libbpfIncludeLinuxDir := fmt.Sprintf("-I%s/../lib/common/libbpf/_include/linux", dir)
	libbpfIncludeUapiLinuxDir := fmt.Sprintf("-I%s/../lib/common/libbpf/_include/uapi/linux", dir)
	libbpfIncludeAsmDir := fmt.Sprintf("-I%s/../lib/common/libbpf/_include/asm", dir)
	targetArch := "-D__TARGET_ARCH_x86"
	if arch == "arm64" {
		targetArch = "-D__TARGET_ARCH_arm64"
	}
	err = loader.CompileEbpf(
		ctx, "-target", "bpf", "-Wall", targetArch, "-g", "-O2", "-c", bpfSourceFile, "-o",
		bpfOutputFile, archLibDir, libbpfSrcDir, libbpfIncludeAsmDir, libbpfIncludeLinuxDir,
		libbpfIncludeUapiLinuxDir, filterDir, conntrackDir, packetparserDir,
	)
	if err != nil {
		return errors.Wrap(err, "failed to compile eBPF")
	}
	p.l.Info("PacketParserTCX compiled")
	return nil
}

func (p *packetParserTCX) Init() error {
	var err error
	if !p.cfg.EnablePodLevel {
		p.l.Warn("packetparsertcx will not init because pod level is disabled")
		return nil
	}

	dir, err := absPath()
	if err != nil {
		return err
	}

	bpfOutputFile := fmt.Sprintf("%s/%s", dir, bpfObjectFileName)

	objs := &packetparsertcxObjects{}
	spec, err := ebpf.LoadCollectionSpec(bpfOutputFile)
	if err != nil {
		return errors.Wrap(err, "failed to load collection spec")
	}
	// packetparsertcxObjects and LoadAndAssign are from bpf2go-generated code;
	// typecheck fails when generated files are not yet built.
	//nolint:typecheck // bpf2go-generated types (packetparsertcxObjects) not present until build
	err = spec.LoadAndAssign(objs, &ebpf.CollectionOptions{
		Maps: ebpf.MapOptions{
			PinPath: plugincommon.MapPath,
		},
	})
	if err != nil { //nolint:typecheck // objs types are from bpf2go-generated code
		p.l.Error("Error loading objects: %w", zap.Error(err))
		return errors.Wrap(err, "failed to load and assign eBPF objects")
	}
	p.objs = objs

	p.endpointIngressInfo, err = p.objs.EndpointIngressFilter.Info()
	if err != nil {
		p.l.Error("Error getting ingress filter info", zap.Error(err))
		return errors.Wrap(err, "failed to get endpoint ingress filter info")
	}
	p.endpointEgressInfo, err = p.objs.EndpointEgressFilter.Info()
	if err != nil {
		p.l.Error("Error getting egress filter info", zap.Error(err))
		return errors.Wrap(err, "failed to get endpoint egress filter info")
	}

	p.hostIngressInfo, err = p.objs.HostIngressFilter.Info()
	if err != nil {
		p.l.Error("Error getting host ingress filter info", zap.Error(err))
		return errors.Wrap(err, "failed to get host ingress filter info")
	}
	p.hostEgressInfo, err = p.objs.HostEgressFilter.Info()
	if err != nil {
		p.l.Error("Error getting host egress filter info", zap.Error(err))
		return errors.Wrap(err, "failed to get host egress filter info")
	}

	p.reader, err = plugincommon.NewPerfReader(p.l, objs.RetinaPacketparserTcxEvents, perCPUBuffer, 1)
	if err != nil {
		p.l.Error("Error NewReader", zap.Error(err))
		return errors.Wrap(err, "failed to create perf reader")
	}

	p.tcxMap = &sync.Map{}
	p.interfaceLockMap = &sync.Map{}

	return nil
}

func (p *packetParserTCX) Start(ctx context.Context) error {
	if !p.cfg.EnablePodLevel {
		p.l.Warn("packetparsertcx will not start because pod level is disabled")
		return nil
	}

	p.l.Info("Starting packetparsertcx")

	p.l.Info("setting up enricher since pod level is enabled")
	if enricher.IsInitialized() {
		p.enricher = enricher.Instance()
	} else {
		p.l.Warn("retina enricher is not initialized")
	}

	ps := pubsub.New()

	fn := pubsub.CallBackFunc(p.endpointWatcherCallbackFn)
	if p.callbackID == "" {
		p.callbackID = ps.Subscribe(common.PubSubEndpoints, &fn)
	}

	if p.cfg.DataAggregationLevel == kcfg.Low {
		p.l.Info("Attaching TCX programs to default interface of k8s Node")
		outgoingLinks, err := utils.GetDefaultOutgoingLinks()
		if err != nil {
			return errors.Wrap(err, "could not get default outgoing links")
		}
		if len(outgoingLinks) == 0 {
			return errNoOutgoingLinks
		}
		outgoingLink := outgoingLinks[0]

		outgoingLinkAttributes := outgoingLink.Attrs()
		p.l.Info("Attaching PacketParserTCX",
			zap.Int("outgoingLink.Index", outgoingLinkAttributes.Index),
			zap.String("outgoingLink.Name", outgoingLinkAttributes.Name),
			zap.Stringer("outgoingLink.HardwareAddr", outgoingLinkAttributes.HardwareAddr),
		)
		p.attachTCXPrograms(*outgoingLink.Attrs(), Device)
	} else {
		p.l.Info("Skipping attaching TCX program to default interface of k8s Node")
	}

	p.recordsChannel = make(chan perf.Record, buffer)
	p.l.Debug("Created records channel")

	return p.run(ctx)
}

func (p *packetParserTCX) Stop() error {
	p.l.Info("Stopping packetparsertcx")

	ps := pubsub.New()

	if p.reader != nil {
		if err := p.reader.Close(); err != nil {
			p.l.Error("Error closing perf reader", zap.Error(err))
		}
	}
	p.l.Debug("Stopped perf reader")

	if p.recordsChannel != nil {
		close(p.recordsChannel)
		p.l.Debug("Closed records channel")
	}

	if p.objs != nil {
		if err := p.objs.Close(); err != nil {
			p.l.Error("Error closing objects", zap.Error(err))
		}
	}
	p.l.Debug("Stopped map/progs")

	if p.callbackID != "" {
		if err := ps.Unsubscribe(common.PubSubEndpoints, p.callbackID); err != nil {
			p.l.Error("Error unregistering callback for packetParserTCX", zap.Error(err))
		}
		p.callbackID = ""
	}

	if err := p.cleanAll(); err != nil {
		p.l.Error("Error cleaning", zap.Error(err))
		return err
	}

	p.l.Info("Stopped packetparsertcx")
	return nil
}

func (p *packetParserTCX) SetupChannel(ch chan *v1.Event) error {
	p.externalChannel = ch
	return nil
}

func (p *packetParserTCX) cleanAll() error {
	if p.tcxMap == nil {
		return nil
	}

	p.tcxMap.Range(func(_ interface{}, value interface{}) bool {
		v := value.(*tcxValue)
		p.cleanTCX(v.ingressLink, v.egressLink)
		return true
	})

	p.tcxMap = &sync.Map{}
	return nil
}

func (p *packetParserTCX) cleanTCX(ingressLink, egressLink link.Link) {
	if ingressLink != nil {
		if err := ingressLink.Close(); err != nil {
			p.l.Debug("could not close ingress TCX link", zap.Error(err))
		}
	}
	if egressLink != nil {
		if err := egressLink.Close(); err != nil {
			p.l.Debug("could not close egress TCX link", zap.Error(err))
		}
	}
}

func (p *packetParserTCX) endpointWatcherCallbackFn(obj interface{}) {
	event := obj.(*endpoint.EndpointEvent)
	if event == nil {
		return
	}

	iface := event.Obj.(netlink.LinkAttrs)

	ifaceKey := ifaceToKey(iface)
	lockMapVal, _ := p.interfaceLockMap.LoadOrStore(ifaceKey, &sync.Mutex{})
	mu := lockMapVal.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()

	switch event.Type {
	case endpoint.EndpointCreated:
		p.l.Debug("Endpoint created", zap.String("name", iface.Name))
		p.attachTCXPrograms(iface, Veth)
	case endpoint.EndpointDeleted:
		p.l.Debug("Endpoint deleted", zap.String("name", iface.Name))
		if value, ok := p.tcxMap.Load(ifaceKey); ok {
			v := value.(*tcxValue)
			p.cleanTCX(v.ingressLink, v.egressLink)
			p.tcxMap.Delete(ifaceKey)
		}
		p.interfaceLockMap.Delete(ifaceKey)
	default:
		p.l.Debug("Unknown event", zap.String("type", event.Type.String()))
	}
}

// attachTCXPrograms attaches BPF programs to an interface using TCX.
func (p *packetParserTCX) attachTCXPrograms(iface netlink.LinkAttrs, ifaceType interfaceType) {
	p.l.Debug("Starting TCX program attachment", zap.String("interface", iface.Name))

	var ingressProgram, egressProgram *ebpf.Program

	switch ifaceType {
	case Device:
		ingressProgram = p.objs.HostIngressFilter
		egressProgram = p.objs.HostEgressFilter
	case Veth:
		ingressProgram = p.objs.EndpointIngressFilter
		egressProgram = p.objs.EndpointEgressFilter
	default:
		p.l.Error("Unknown interface type", zap.String("interface type", string(ifaceType)))
		return
	}

	ingressLink, err := link.AttachTCX(link.TCXOptions{
		Program:   ingressProgram,
		Attach:    ebpf.AttachTCXIngress,
		Interface: iface.Index,
		Anchor:    link.Head(),
	})
	if err != nil {
		p.l.Error("could not attach TCX ingress program",
			zap.String("interface", iface.Name), zap.Error(err))
		return
	}

	egressLink, err := link.AttachTCX(link.TCXOptions{
		Program:   egressProgram,
		Attach:    ebpf.AttachTCXEgress,
		Interface: iface.Index,
		Anchor:    link.Head(),
	})
	if err != nil {
		ingressLink.Close() //nolint:errcheck // best effort cleanup
		p.l.Error("could not attach TCX egress program",
			zap.String("interface", iface.Name), zap.Error(err))
		return
	}

	ifaceKey := ifaceToKey(iface)
	p.tcxMap.Store(ifaceKey, &tcxValue{
		ingressLink: ingressLink,
		egressLink:  egressLink,
	})

	p.l.Debug("Successfully attached BPF programs using TCX", zap.String("interface", iface.Name))
}

func (p *packetParserTCX) run(ctx context.Context) error {
	for i := 0; i < workers; i++ {
		p.wg.Add(1)
		go p.processRecord(ctx, i)
	}
	go p.handleEvents(ctx)

	p.l.Info("Started packetparsertcx")

	p.wg.Wait()
	p.l.Info("All workers have stopped")

	p.l.Info("Context is done, packetparsertcx will stop running")
	return nil
}

func (p *packetParserTCX) processRecord(ctx context.Context, id int) {
	defer p.wg.Done()
	for {
		select {
		case <-ctx.Done():
			p.l.Info("Context is done, stopping Worker", zap.Int("worker_id", id))
			return
		case record := <-p.recordsChannel:
			p.l.Debug("Received record",
				zap.Int("cpu", record.CPU),
				zap.Uint64("lost_samples", record.LostSamples),
				zap.Int("bytes_remaining", record.Remaining),
				zap.Int("worker_id", id),
			)

			metrics.ParsedPacketsCounter.WithLabelValues().Inc()

			var bpfEvent packetparsertcxPacket
			err := binary.Read(bytes.NewReader(record.RawSample), binary.LittleEndian, &bpfEvent)
			if err != nil {
				p.l.Error("Error reading bpfEvent", zap.Error(err))
				continue
			}

			sourcePortShort := uint32(utils.HostToNetShort(bpfEvent.SrcPort))
			destinationPortShort := uint32(utils.HostToNetShort(bpfEvent.DstPort))

			// Cast T_nsec to int64. While T_nsec is uint64, kernel timestamps fit safely in int64 range.
			//nolint:gosec // G115: T_nsec represents kernel nanoseconds which are always within int64 range
			timestamp := ktime.MonotonicOffset.Nanoseconds() + int64(bpfEvent.T_nsec)
			fl := utils.ToFlow(
				p.l,
				timestamp,
				utils.Int2ip(bpfEvent.SrcIp).To4(),
				utils.Int2ip(bpfEvent.DstIp).To4(),
				sourcePortShort,
				destinationPortShort,
				bpfEvent.Proto,
				bpfEvent.ObservationPoint,
				flow.Verdict_FORWARDED,
			)
			if fl == nil {
				p.l.Warn("Could not convert bpfEvent to flow", zap.Any("bpfEvent", bpfEvent))
				continue
			}

			fl.IsReply = &wrapperspb.BoolValue{Value: bpfEvent.IsReply}
			fl.TrafficDirection = flow.TrafficDirection(bpfEvent.TrafficDirection)

			ext := utils.NewExtensions()

			utils.AddPacketSize(ext, bpfEvent.Bytes)
			utils.AddPreviouslyObservedBytes(ext, bpfEvent.PreviouslyObservedBytes)
			utils.AddPreviouslyObservedPackets(ext, bpfEvent.PreviouslyObservedPackets)

			tcpMetadata := bpfEvent.TcpMetadata
			utils.AddTCPFlags(
				fl,
				uint16((bpfEvent.Flags&TCPFlagSYN)>>1),
				uint16((bpfEvent.Flags&TCPFlagACK)>>4), // nolint:gomnd // 4 is the offset for ACK.
				uint16((bpfEvent.Flags&TCPFlagFIN)>>0),
				uint16((bpfEvent.Flags&TCPFlagRST)>>2), // nolint:gomnd // 2 is the offset for RST.
				uint16((bpfEvent.Flags&TCPFlagPSH)>>3), // nolint:gomnd // 3 is the offset for PSH.
				uint16((bpfEvent.Flags&TCPFlagURG)>>5), // nolint:gomnd // 5 is the offset for URG.
				uint16((bpfEvent.Flags&TCPFlagECE)>>6), // nolint:gomnd // 6 is the offset for ECE.
				uint16((bpfEvent.Flags&TCPFlagCWR)>>7), // nolint:gomnd // 7 is the offset for CWR.
				uint16((bpfEvent.Flags&TCPFlagNS)>>8),  // nolint:gomnd // 8 is the offset for NS.
			)
			utils.AddPreviouslyObservedTCPFlags(
				ext,
				bpfEvent.PreviouslyObservedFlags.Syn,
				bpfEvent.PreviouslyObservedFlags.Ack,
				bpfEvent.PreviouslyObservedFlags.Fin,
				bpfEvent.PreviouslyObservedFlags.Rst,
				bpfEvent.PreviouslyObservedFlags.Psh,
				bpfEvent.PreviouslyObservedFlags.Urg,
				bpfEvent.PreviouslyObservedFlags.Ece,
				bpfEvent.PreviouslyObservedFlags.Cwr,
				bpfEvent.PreviouslyObservedFlags.Ns,
			)

			if fl.GetTraceObservationPoint() == flow.TraceObservationPoint_TO_NETWORK {
				utils.AddTCPID(ext, uint64(tcpMetadata.Tsval))
			} else if fl.GetTraceObservationPoint() == flow.TraceObservationPoint_FROM_NETWORK {
				utils.AddTCPID(ext, uint64(tcpMetadata.Tsecr))
			}

			utils.SetExtensions(fl, ext)

			ev := &v1.Event{
				Event:     fl,
				Timestamp: fl.GetTime(),
			}
			if p.enricher != nil {
				p.enricher.Write(ev)
			}

			if p.externalChannel != nil {
				select {
				case p.externalChannel <- ev:
				default:
					metrics.LostEventsCounter.WithLabelValues(utils.ExternalChannel, name).Inc()
				}
			}
		}
	}
}

func (p *packetParserTCX) handleEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			p.l.Info("Context is done, stopping handleEvents")
			return
		default:
			p.readData()
		}
	}
}

func (p *packetParserTCX) readData() {
	record, err := p.reader.Read()
	if err != nil {
		if errors.Is(err, perf.ErrClosed) {
			p.l.Error("Perf array is empty")
		} else {
			p.l.Error("Error reading perf array", zap.Error(err))
		}
		return
	}

	if record.LostSamples > 0 {
		metrics.LostEventsCounter.WithLabelValues(utils.Kernel, name).Add(float64(record.LostSamples))
		return
	}

	select {
	case p.recordsChannel <- record:
	default:
		metrics.LostEventsCounter.WithLabelValues(utils.BufferedChannel, name).Inc()
	}
}

func absPath() (string, error) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("failed to determine current file path")
	}
	dir := path.Dir(filename)
	return dir, nil
}

// IsTCXSupported probes whether the kernel supports TCX attachment by attempting
// a test attach on the loopback interface. This can be called before activating
// the plugin to decide whether to use TCX or fall back to TC.
func IsTCXSupported() bool {
	links, err := netlink.LinkList()
	if err != nil || len(links) == 0 {
		return false
	}

	var loopback netlink.Link
	for _, l := range links {
		if l.Attrs().Name == "lo" {
			loopback = l
			break
		}
	}
	if loopback == nil {
		return false
	}

	progSpec := &ebpf.ProgramSpec{
		Type:       ebpf.SchedCLS,
		AttachType: ebpf.AttachTCXIngress,
		License:    "Dual MIT/GPL",
		Instructions: asm.Instructions{
			asm.Mov.Imm(asm.R0, -1),
			asm.Return(),
		},
	}

	prog, err := ebpf.NewProgram(progSpec)
	if err != nil {
		return false
	}
	defer prog.Close()

	testLink, err := link.AttachTCX(link.TCXOptions{
		Program:   prog,
		Attach:    ebpf.AttachTCXIngress,
		Interface: loopback.Attrs().Index,
	})
	if err != nil {
		return false
	}

	testLink.Close() //nolint:errcheck // test probe cleanup
	return true
}
