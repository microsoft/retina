// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

// package packetparser contains the Retina packetparser plugin. It utilizes eBPF to parse packets and generate flow events.
package packetparser

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"path"
	"runtime"
	"sync"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/cilium/cilium/api/v1/flow"
	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/perf"
	tc "github.com/florianl/go-tc"
	helper "github.com/florianl/go-tc/core"
	nl "github.com/mdlayher/netlink"
	"github.com/microsoft/retina/internal/ktime"
	"github.com/microsoft/retina/pkg/common"
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/loader"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/metrics"
	"github.com/microsoft/retina/pkg/plugin/api"
	plugincommon "github.com/microsoft/retina/pkg/plugin/common"
	_ "github.com/microsoft/retina/pkg/plugin/lib/_amd64"                            // nolint
	_ "github.com/microsoft/retina/pkg/plugin/lib/_arm64"                            // nolint
	_ "github.com/microsoft/retina/pkg/plugin/lib/common/libbpf/_include/asm"        // nolint
	_ "github.com/microsoft/retina/pkg/plugin/lib/common/libbpf/_include/linux"      // nolint
	_ "github.com/microsoft/retina/pkg/plugin/lib/common/libbpf/_include/uapi/linux" // nolint
	_ "github.com/microsoft/retina/pkg/plugin/lib/common/libbpf/_src"                // nolint
	_ "github.com/microsoft/retina/pkg/plugin/packetparser/_cprog"                   // nolint
	"github.com/microsoft/retina/pkg/pubsub"
	"github.com/microsoft/retina/pkg/utils"
	"github.com/microsoft/retina/pkg/watchers/endpoint"
	"github.com/vishvananda/netlink"
	vnl "github.com/vishvananda/netlink/nl"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
)

//go:generate bpf2go -cflags "-g -O2 -Wall -D__TARGET_ARCH_${GOARCH} -Wall" -target ${GOARCH} -type packet packetparser ./_cprog/packetparser.c -- -I../lib/_${GOARCH} -I../lib/common/libbpf/_src -I../lib/common/libbpf/_include/linux -I../lib/common/libbpf/_include/uapi/linux -I../lib/common/libbpf/_include/asm -I../filter/_cprog/ -I../conntrack/_cprog/
var errNoOutgoingLinks = errors.New("could not determine any outgoing links")

// New creates a packetparser plugin.
func New(cfg *kcfg.Config) api.Plugin {
	return &packetParser{
		cfg: cfg,
		l:   log.Logger().Named(string(Name)),
	}
}

func (p *packetParser) Name() string {
	return string(Name)
}

func (p *packetParser) Generate(ctx context.Context) error {
	// Get absolute path to this file during runtime.
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return errors.New("unable to get absolute path to this file")
	}
	dir := path.Dir(filename)
	dynamicHeaderPath := fmt.Sprintf("%s/%s/%s", dir, bpfSourceDir, dynamicHeaderFileName)
	// Check if packetparser will bypassing lookup IP of interest.
	bypassLookupIPOfInterest := 0
	if p.cfg.BypassLookupIPOfInterest {
		p.l.Info("bypassing lookup IP of interest")
		bypassLookupIPOfInterest = 1
	}
	p.l.Info("data aggregation level", zap.String("level", p.cfg.DataAggregationLevel.String()))
	st := fmt.Sprintf("#define BYPASS_LOOKUP_IP_OF_INTEREST %d\n#define DATA_AGGREGATION_LEVEL %d\n", bypassLookupIPOfInterest, p.cfg.DataAggregationLevel)
	err := loader.WriteFile(ctx, dynamicHeaderPath, st)
	if err != nil {
		return errors.Wrap(err, "failed to write dynamic header")
	}
	p.l.Info("PacketParser header generated at", zap.String("path", dynamicHeaderPath))
	return nil
}

func (p *packetParser) Compile(ctx context.Context) error {
	// Get the absolute path to this file during runtime.
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
	libbpfSrcDir := fmt.Sprintf("-I%s/../lib/common/libbpf/_src", dir)
	libbpfIncludeLinuxDir := fmt.Sprintf("-I%s/../lib/common/libbpf/_include/linux", dir)
	libbpfIncludeUapiLinuxDir := fmt.Sprintf("-I%s/../lib/common/libbpf/_include/uapi/linux", dir)
	libbpfIncludeAsmDir := fmt.Sprintf("-I%s/../lib/common/libbpf/_include/asm", dir)
	targetArch := "-D__TARGET_ARCH_x86"
	if arch == "arm64" {
		targetArch = "-D__TARGET_ARCH_arm64"
	}
	// Keep target as bpf, otherwise clang compilation yields bpf object that elf reader cannot load.
	err = loader.CompileEbpf(ctx, "-target", "bpf", "-Wall", targetArch, "-g", "-O2", "-c", bpfSourceFile, "-o", bpfOutputFile,
		archLibDir,
		libbpfSrcDir,
		libbpfIncludeAsmDir,
		libbpfIncludeLinuxDir,
		libbpfIncludeUapiLinuxDir,
		filterDir,
		conntrackDir,
	)
	if err != nil {
		return err
	}
	p.l.Info("PacketParser metric compiled")
	return nil
}

func (p *packetParser) Init() error {
	var err error
	if !p.cfg.EnablePodLevel {
		p.l.Warn("packet parser and latency plugin will not init because pod level is disabled")
		return nil
	}
	// Get the absolute path to this file during runtime.
	dir, err := absPath()
	if err != nil {
		return err
	}

	bpfOutputFile := fmt.Sprintf("%s/%s", dir, bpfObjectFileName)

	objs := &packetparserObjects{}
	spec, err := ebpf.LoadCollectionSpec(bpfOutputFile)
	if err != nil {
		return err
	}
	//nolint:typecheck
	if err := spec.LoadAndAssign(objs, &ebpf.CollectionOptions{ //nolint:typecheck
		Maps: ebpf.MapOptions{
			PinPath: plugincommon.MapPath,
		},
	}); err != nil { //nolint:typecheck
		p.l.Error("Error loading objects: %w", zap.Error(err))
		return err
	}
	p.objs = objs

	// Endpoint bpf programs.
	p.endpointIngressInfo, err = p.objs.EndpointIngressFilter.Info()
	if err != nil {
		p.l.Error("Error getting ingress filter info", zap.Error(err))
		return err
	}
	p.endpointEgressInfo, err = p.objs.EndpointEgressFilter.Info()
	if err != nil {
		p.l.Error("Error getting egress filter info", zap.Error(err))
		return err
	}

	// Host bpf programs.
	p.hostIngressInfo, err = p.objs.HostIngressFilter.Info()
	if err != nil {
		p.l.Error("Error getting ingress filter info", zap.Error(err))
		return err
	}
	p.hostEgressInfo, err = p.objs.HostEgressFilter.Info()
	if err != nil {
		p.l.Error("Error getting egress filter info", zap.Error(err))
		return err
	}

	p.reader, err = plugincommon.NewPerfReader(p.l, objs.RetinaPacketparserEvents, perCPUBuffer, 1)
	if err != nil {
		p.l.Error("Error NewReader", zap.Error(err))
		return err
	}

	p.tcMap = &sync.Map{}
	p.interfaceLockMap = &sync.Map{}

	return nil
}

func (p *packetParser) Start(ctx context.Context) error {
	if !p.cfg.EnablePodLevel {
		p.l.Warn("packet parser and latency plugin will not start because pod level is disabled")
		return nil
	}

	p.l.Info("Starting packet parser")

	p.l.Info("setting up enricher since pod level is enabled")
	// Set up enricher.
	if enricher.IsInitialized() {
		p.enricher = enricher.Instance()
	} else {
		p.l.Warn("retina enricher is not initialized")
	}

	// Get Pubsub instance.
	ps := pubsub.New()

	// Register callback.
	// Every time a new endpoint is created, we will create a qdisc and attach it to the endpoint.
	fn := pubsub.CallBackFunc(p.endpointWatcherCallbackFn)
	// Check if callback is already registered.
	if p.callbackID == "" {
		p.callbackID = ps.Subscribe(common.PubSubEndpoints, &fn)
	}

	if p.cfg.DataAggregationLevel == kcfg.Low {
		p.l.Info("Attaching bpf program to default interface of k8s Node in node namespace")
		outgoingLinks, err := utils.GetDefaultOutgoingLinks()
		if err != nil {
			return errors.Wrap(err, "could not get default outgoing links")
		}
		if len(outgoingLinks) == 0 {
			return errNoOutgoingLinks
		}
		outgoingLink := outgoingLinks[0] // Take first link until multi-link support is implemented

		outgoingLinkAttributes := outgoingLink.Attrs()
		p.l.Info("Attaching Packetparser",
			zap.Int("outgoingLink.Index", outgoingLinkAttributes.Index),
			zap.String("outgoingLink.Name", outgoingLinkAttributes.Name),
			zap.Stringer("outgoingLink.HardwareAddr", outgoingLinkAttributes.HardwareAddr),
		)
		p.createQdiscAndAttach(*outgoingLink.Attrs(), Device)
	} else {
		p.l.Info("Skipping attaching bpf program to default interface of k8s Node in node namespace")
	}

	// Create the channel.
	p.recordsChannel = make(chan perf.Record, buffer)
	p.l.Debug("Created records channel")

	return p.run(ctx)
}

func (p *packetParser) Stop() error {
	p.l.Info("Stopping packet parser")

	// Get pubsubs instance
	ps := pubsub.New()

	// Stop perf reader.
	if p.reader != nil {
		if err := p.reader.Close(); err != nil {
			p.l.Error("Error closing perf reader", zap.Error(err))
		}
	}
	p.l.Debug("Stopped perf reader")

	// Close the channel. The producer should have stopped by now.
	// All consumers should have stopped by now.
	if p.recordsChannel != nil {
		close(p.recordsChannel)
		p.l.Debug("Closed records channel")
	}

	// Stop map and progs.
	if p.objs != nil {
		if err := p.objs.Close(); err != nil {
			p.l.Error("Error closing objects", zap.Error(err))
		}
	}
	p.l.Debug("Stopped map/progs")

	// Unregister callback.
	if p.callbackID != "" {
		if err := ps.Unsubscribe(common.PubSubEndpoints, p.callbackID); err != nil {
			p.l.Error("Error unregistering callback for packetParser", zap.Error(err))
		}
		// Reset callback ID.
		p.callbackID = ""
	}

	if err := p.cleanAll(); err != nil {
		p.l.Error("Error cleaning", zap.Error(err))
		return err
	}

	p.l.Info("Stopped packet parser")
	return nil
}

func (p *packetParser) SetupChannel(ch chan *v1.Event) error {
	p.externalChannel = ch
	return nil
}

// cleanAll is NOT thread safe.
// Not required for now.
func (p *packetParser) cleanAll() error {
	// Delete tunnel and qdiscs.
	if p.tcMap == nil {
		return nil
	}

	p.tcMap.Range(func(key, value interface{}) bool {
		v := value.(*tcValue)
		p.clean(v.tc, v.qdisc)
		return true
	})

	// Reset map.
	// It is OK to do this without a lock because
	// cleanAll is only invoked from Stop(), and Stop can
	// only be called from PluginManager (which is single threaded).
	p.tcMap = &sync.Map{}
	return nil
}

func (p *packetParser) clean(rtnl nltc, qdisc *tc.Object) {
	// Warning, not error. Clean is best effort.
	if rtnl != nil {
		if err := getQdisc(rtnl).Delete(qdisc); err != nil && !errors.Is(err, tc.ErrNoArg) {
			p.l.Debug("could not delete egress qdisc", zap.Error(err))
		}
		if err := rtnl.Close(); err != nil {
			p.l.Warn("could not close rtnetlink socket", zap.Error(err))
		}
	}
}

func (p *packetParser) endpointWatcherCallbackFn(obj interface{}) {
	// Contract is that we will receive an endpoint event pointer.
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
		p.createQdiscAndAttach(iface, Veth)
	case endpoint.EndpointDeleted:
		p.l.Debug("Endpoint deleted", zap.String("name", iface.Name))
		// Clean.
		if value, ok := p.tcMap.Load(ifaceKey); ok {
			v := value.(*tcValue)
			p.clean(v.tc, v.qdisc)
			// Delete from map.
			p.tcMap.Delete(ifaceKey)
		}
	default:
		// Unknown.
		p.l.Debug("Unknown event", zap.String("type", event.Type.String()))
	}
}

// createQdiscAndAttach creates a qdisc of type clsact on the interface and attaches the ingress and egress bpf filter programs to it.
// Only support interfaces of type veth and device.
func (p *packetParser) createQdiscAndAttach(iface netlink.LinkAttrs, ifaceType interfaceType) {
	p.l.Debug("Starting qdisc attachment", zap.String("interface", iface.Name))

	var (
		ingressProgram, egressProgram *ebpf.Program
		ingressInfo, egressInfo       *ebpf.ProgramInfo
		err                           error
	)

	switch ifaceType {
	case Device:
		ingressProgram = p.objs.HostIngressFilter
		egressProgram = p.objs.HostEgressFilter
		ingressInfo = p.hostIngressInfo
		egressInfo = p.hostEgressInfo
	case Veth:
		ingressProgram = p.objs.EndpointIngressFilter
		egressProgram = p.objs.EndpointEgressFilter
		ingressInfo = p.endpointIngressInfo
		egressInfo = p.endpointEgressInfo
	default:
		p.l.Error("Unknown interface type", zap.String("interface type", string(ifaceType)))
		return
	}

	// open a rtnetlink socket
	rtnl, err := tcOpen(&tc.Config{})
	if err != nil {
		p.l.Error("could not open rtnetlink socket", zap.Int("NetNsID", iface.NetNsID), zap.Error(err))
		return
	}
	// set extended acknowledge option for more detailed error messages.
	if err = rtnl.SetOption(nl.ExtendedAcknowledge, true); err != nil {
		p.l.Warn("could not set extended acknowledge option", zap.Error(err))
	}

	// Create a qdisc of type clsact on the tunnel interface.
	// Even though the parameter we are setting is for ingress, clact is a special qdisc that have both ingress and egress hooks.
	// https://lwn.net/Articles/671458/
	// We will attach the ingress and egress programs to this qdisc.
	clsactQdisc := &tc.Object{
		Msg: tc.Msg{
			Family:  unix.AF_UNSPEC,
			Ifindex: uint32(iface.Index),
			Handle:  helper.BuildHandle(0xFFFF, 0x0000), // nolint:gomnd // special handle for ingress qdisc https://tldp.org/HOWTO/Traffic-Control-HOWTO/components.html
			// we can actually be pedantic and create another qdisc for egress by setting the parent to tc.HandleRoot but it is not necessary for reasons stated above.
			Parent: tc.HandleIngress,
		},
		Attribute: tc.Attribute{
			Kind: "clsact",
		},
	}
	defer func() {
		if err != nil {
			p.clean(rtnl, clsactQdisc)
		}
	}()
	// Install Qdisc on interface.
	if err := getQdisc(rtnl).Add(clsactQdisc); err != nil && !errors.Is(err, os.ErrExist) {
		p.l.Error("could not assign clsact to tunnel interface", zap.String("interface", iface.Name), zap.Error(err))
		return
	}
	// Create an ingress filter of type bpf on the tunnel interface.
	ingressFilter := tc.Object{
		Msg: tc.Msg{
			Family:  unix.AF_UNSPEC,
			Ifindex: uint32(iface.Index),
			Handle:  0,                                                                // arbitrary handle to distinguish between ingress and egress filters
			Parent:  helper.BuildHandle(0xFFFF, tc.HandleMinIngress),                  // nolint:gomnd // same major component (0xffff) as clsact
			Info:    netlink.MakeHandle(tcFilterPriority, vnl.Swap16(unix.ETH_P_ALL)), // set the filter priority and protocol to ETH_P_ALL
		},
		Attribute: tc.Attribute{
			Kind: "bpf",
			BPF: &tc.Bpf{
				FD:    utils.Uint32Ptr(uint32(getFD(ingressProgram))),
				Name:  utils.StringPtr(ingressInfo.Name),
				Flags: utils.Uint32Ptr(0x1),
			},
		},
	}
	if err := getFilter(rtnl).Add(&ingressFilter); err != nil && !errors.Is(err, os.ErrExist) {
		p.l.Error("could not add bpf ingress filter to qdisc", zap.String("interface", iface.Name), zap.Error(err))
		return
	}
	// Create an egress filter of type bpf on the endpoint interface.
	egressFilter := tc.Object{
		Msg: tc.Msg{
			Family:  unix.AF_UNSPEC,
			Ifindex: uint32(iface.Index),
			Handle:  1,
			Parent:  helper.BuildHandle(0xFFFF, tc.HandleMinEgress),                   // nolint:gomnd // ignore
			Info:    netlink.MakeHandle(tcFilterPriority, vnl.Swap16(unix.ETH_P_ALL)), // nolint:gomnd // ignore
		},
		Attribute: tc.Attribute{
			Kind: "bpf",
			BPF: &tc.Bpf{
				FD:    utils.Uint32Ptr(uint32(getFD(egressProgram))),
				Name:  utils.StringPtr(egressInfo.Name),
				Flags: utils.Uint32Ptr(0x1),
			},
		},
	}
	if err := getFilter(rtnl).Add(&egressFilter); err != nil && !errors.Is(err, os.ErrExist) {
		p.l.Error("could not add bpf egress filter to qdisc", zap.String("interface", iface.Name), zap.Error(err))
		return
	}

	// Cache.
	ifaceKey := ifaceToKey(iface)
	tcValue := &tcValue{
		tc:    rtnl,
		qdisc: clsactQdisc,
	}
	p.tcMap.Store(ifaceKey, tcValue)

	p.l.Debug("Successfully added bpf", zap.String("interface", iface.Name))
}

func (p *packetParser) run(ctx context.Context) error {
	// Start perf record handlers (consumers).
	for i := 0; i < workers; i++ {
		p.wg.Add(1)
		go p.processRecord(ctx, i)
	}
	// Start events handler from perf array in kernel (producer).
	// Don't add it to the wait group because we don't want to wait for it.
	// The perf reader Read call blocks until there is data available in the perf buffer.
	// That call is unblocked when Reader is closed.
	go p.handleEvents(ctx)

	p.l.Info("Started packet parser")

	// Wait for the context to be done.
	// This will block till all consumers exit.
	p.wg.Wait()
	p.l.Info("All workers have stopped")

	p.l.Info("Context is done, packet parser will stop running")
	return nil
}

// This is the data consumer.
// There will more than one of these.
func (p *packetParser) processRecord(ctx context.Context, id int) {
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

			var bpfEvent packetparserPacket
			err := binary.Read(bytes.NewReader(record.RawSample), binary.LittleEndian, &bpfEvent)
			if err != nil {
				p.l.Error("Error reading bpfEvent", zap.Error(err))
				continue
			}

			// Post processing of the bpfEvent.
			// Anything after this is required only for Pod level metrics.
			sourcePortShort := uint32(utils.HostToNetShort(bpfEvent.SrcPort))
			destinationPortShort := uint32(utils.HostToNetShort(bpfEvent.DstPort))

			fl := utils.ToFlow(
				p.l,
				ktime.MonotonicOffset.Nanoseconds()+int64(bpfEvent.T_nsec),
				utils.Int2ip(bpfEvent.SrcIp).To4(), // Precautionary To4() call.
				utils.Int2ip(bpfEvent.DstIp).To4(), // Precautionary To4() call.
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

			// Add the isReply flag to the flow.
			fl.IsReply = &wrapperspb.BoolValue{Value: bpfEvent.IsReply}

			// Add the traffic direction to the flow.
			fl.TrafficDirection = flow.TrafficDirection(bpfEvent.TrafficDirection)

			meta := &utils.RetinaMetadata{}

			// Add packet size to the flow's metadata.
			utils.AddPacketSize(meta, bpfEvent.Bytes)

			// Add the TCP metadata to the flow.
			tcpMetadata := bpfEvent.TcpMetadata
			utils.AddTCPFlags(
				fl,
				uint16((bpfEvent.Flags&TCPFlagSYN)>>1),
				uint16((bpfEvent.Flags&TCPFlagACK)>>4), // nolint:gomnd // 4 is the offset for ACK.
				uint16((bpfEvent.Flags&TCPFlagFIN)>>0),
				uint16((bpfEvent.Flags&TCPFlagRST)>>2), // nolint:gomnd // 2 is the offset for RST.
				uint16((bpfEvent.Flags&TCPFlagPSH)>>3), // nolint:gomnd // 3 is the offset for PSH.
				uint16((bpfEvent.Flags&TCPFlagURG)>>5), // nolint:gomnd // 5 is the offset for URG.
			)

			// For packets originating from node, we use tsval as the tcpID.
			// Packets coming back has the tsval echoed in tsecr.
			if fl.GetTraceObservationPoint() == flow.TraceObservationPoint_TO_NETWORK {
				utils.AddTCPID(meta, uint64(tcpMetadata.Tsval))
			} else if fl.GetTraceObservationPoint() == flow.TraceObservationPoint_FROM_NETWORK {
				utils.AddTCPID(meta, uint64(tcpMetadata.Tsecr))
			}

			// Add metadata to the flow.
			utils.AddRetinaMetadata(fl, meta)

			// Write the event to the enricher.
			ev := &v1.Event{
				Event:     fl,
				Timestamp: fl.GetTime(),
			}
			if p.enricher != nil {
				p.enricher.Write(ev)
			}

			// Write the event to the external channel.
			if p.externalChannel != nil {
				select {
				case p.externalChannel <- ev:
				default:
					// Channel is full, drop the event.
					// We shouldn't slow down the reader.
					metrics.LostEventsCounter.WithLabelValues(utils.ExternalChannel, string(Name)).Inc()
				}
			}
		}
	}
}

func (p *packetParser) handleEvents(ctx context.Context) {
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

// This is the data producer.
func (p *packetParser) readData() {
	// Read call blocks until there is data available in the perf buffer.
	// This is unblocked by the close call.
	record, err := p.reader.Read()
	if err != nil {
		if errors.Is(err, perf.ErrClosed) {
			p.l.Error("Perf array is empty")
			// nothing to do, we're done
		} else {
			p.l.Error("Error reading perf array", zap.Error(err))
		}
		return
	}

	if record.LostSamples > 0 {
		// p.l.Warn("Lostsamples", zap.Uint64("lost samples", record.LostSamples))
		metrics.LostEventsCounter.WithLabelValues(utils.Kernel, string(Name)).Add(float64(record.LostSamples))
		return
	}

	select {
	case p.recordsChannel <- record:
	default:
		// Channel is full, drop the record.
		// We shouldn't slow down the perf array reader.
		metrics.LostEventsCounter.WithLabelValues(utils.BufferedChannel, string(Name)).Inc()
	}
}

// Helper functions.

// absPath returns the absolute path to the directory where this file resides.
func absPath() (string, error) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("failed to determine current file path")
	}
	dir := path.Dir(filename)
	return dir, nil
}
