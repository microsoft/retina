// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

// Package dropreason contains the Retina DropReason plugin. It measures the number of packets/bytes dropped and the reason and the direction of the drop.
package dropreason

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"path"
	"runtime"
	"time"

	"github.com/cilium/cilium/api/v1/flow"
	hubblev1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
	"github.com/microsoft/retina/internal/ktime"
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/loader"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/metrics"
	"github.com/microsoft/retina/pkg/plugin/api"
	plugincommon "github.com/microsoft/retina/pkg/plugin/common"
	_ "github.com/microsoft/retina/pkg/plugin/dropreason/_cprog" // nolint
	"github.com/microsoft/retina/pkg/utils"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

//go:generate bpf2go -cflags "-g -O2 -Wall -D__TARGET_ARCH_${GOARCH} -Wall" -target ${GOARCH} -type metrics_map_value -type drop_reason_t -type packet kprobe ./_cprog/drop_reason.c -- -I../lib/_${GOARCH} -I../lib/common/libbpf/_src -I../filter/_cprog/
const (
	nfHookSlowFn         = "nf_hook_slow"
	tcpConnectFn         = "tcp_v4_connect"
	intCskAcceptFn       = "inet_csk_accept"
	nfNatInetFn          = "nf_nat_inet_fn"
	nfConntrackConfirmFn = "__nf_conntrack_confirm"
)

// New creates a new dropreason plugin.
// When opts.EnablePodLevel=false, the enricher will not be used.
func New(cfg *kcfg.Config) api.Plugin {
	return &dropReason{
		cfg: cfg,
		l:   log.Logger().Named(string(Name)),
	}
}

// Plugin API implementation for packet forward.
// Ref: github.com/microsoft/retina/pkg/plugin/api

func (dr *dropReason) Name() string {
	return string(Name)
}

func (dr *dropReason) Generate(ctx context.Context) error {
	// Get absolute path to this file during runtime.
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return errors.New("unable to get absolute path to this file")
	}
	dir := path.Dir(filename)
	dynamicHeaderPath := fmt.Sprintf("%s/%s/%s", dir, bpfSourceDir, dynamicHeaderFileName)
	i := 0
	if dr.cfg.EnablePodLevel {
		i = 1
	}

	j := 0
	if dr.cfg.BypassLookupIPOfInterest {
		dr.l.Logger.Info("Bypassing lookup IP of interest")
		j = 1
	}
	st := fmt.Sprintf("#define ADVANCED_METRICS %d \n#define BYPASS_LOOKUP_IP_OF_INTEREST %d \n", i, j)
	err := loader.WriteFile(ctx, dynamicHeaderPath, st)
	if err != nil {
		dr.l.Error("Error writing dynamic header", zap.Error(err))
		return err
	}

	dr.l.Debug("DropReason header generated at", zap.String("path", dynamicHeaderPath))
	return nil
}

// Compile should be able to compile the eBPF source code during runtime.
func (dr *dropReason) Compile(ctx context.Context) error {
	// Get the absolute path to this file during runtime.
	dir, err := absPath()
	if err != nil {
		return err
	}

	bpfSourceFile := fmt.Sprintf("%s/%s/%s", dir, bpfSourceDir, bpfSourceFileName)
	bpfOutputFile := fmt.Sprintf("%s/%s", dir, bpfObjectFileName)

	arch := runtime.GOARCH
	includeDir := fmt.Sprintf("-I%s/../lib/_%s", dir, arch)
	filterDir := fmt.Sprintf("-I%s/../filter/_cprog/", dir)
	libbpfDir := fmt.Sprintf("-I%s/../lib/common/libbpf/_src", dir)
	targetArch := "-D__TARGET_ARCH_x86"
	if arch == "arm64" {
		targetArch = "-D__TARGET_ARCH_arm64"
	}
	// Keep target as bpf, otherwise clang compilation yields bpf object that elf reader cannot load.
	err = loader.CompileEbpf(ctx, "-target", "bpf", "-Wall", targetArch, "-g", "-O2", "-c", bpfSourceFile, "-o", bpfOutputFile, includeDir, libbpfDir, filterDir)
	if err != nil {
		return errors.Wrap(err, "unable to compile eBPF code")
	}
	dr.l.Info("DropReason metric compiled")
	return nil
}

func (dr *dropReason) Init() error {
	var err error
	// Get the absolute path to this file during runtime.
	dir, err := absPath()
	if err != nil {
		return err
	}

	bpfOutputFile := fmt.Sprintf("%s/%s", dir, bpfObjectFileName)

	objs := &kprobeObjects{} //nolint:typecheck
	spec, err := ebpf.LoadCollectionSpec(bpfOutputFile)
	if err != nil {
		return err
	}

	// TODO remove the opts
	if err := spec.LoadAndAssign(objs, &ebpf.CollectionOptions{
		Programs: ebpf.ProgramOptions{
			LogLevel: 2,
		},
		Maps: ebpf.MapOptions{
			PinPath: plugincommon.MapPath,
		},
	}); err != nil {
		dr.l.Error("Error loading objects: %w", zap.Error(err))
		return err
	}

	// read perf map
	dr.reader, err = plugincommon.NewPerfReader(dr.l, objs.RetinaDropreasonEvents, perCPUBuffer, 1)
	if err != nil {
		dr.l.Error("Error NewReader: %w", zap.Error(err))
		return err
	}

	dr.KNfHook, err = link.Kprobe(nfHookSlowFn, objs.NfHookSlow, nil)
	if err != nil {
		dr.l.Error("opening kprobe: %w", zap.Error(err))
		return err
	}

	dr.KRetnfhook, err = link.Kretprobe(nfHookSlowFn, objs.NfHookSlowRet, nil)
	if err != nil {
		dr.l.Error("opening kretprobe: %w", zap.Error(err))
		return err
	}

	dr.KRetTCPConnect, err = link.Kretprobe(tcpConnectFn, objs.TcpV4ConnectRet, nil)
	if err != nil {
		dr.l.Error("opening kretprobe: %w", zap.Error(err))
		return err
	}

	dr.KTCPAccept, err = link.Kretprobe(intCskAcceptFn, objs.InetCskAccept, nil)
	if err != nil {
		dr.l.Error("opening kretprobe: %w", zap.Error(err))
		return err
	}

	dr.KRetTCPAccept, err = link.Kretprobe(intCskAcceptFn, objs.InetCskAcceptRet, nil)
	if err != nil {
		dr.l.Error("opening kretprobe: %w", zap.Error(err))
		return err
	}

	dr.KNfNatInet, err = link.Kretprobe(nfNatInetFn, objs.NfNatInetFn, nil)
	if err != nil {
		// TODO: remove this check once we get this working on Mariner OS.
		if errors.Is(err, os.ErrNotExist) {
			dr.l.Warn("nf_nat_inet_fn not found, skipping attaching kretprobe to it. This may impact the drop reason metrics.")
		} else {
			dr.l.Error("opening kretprobe: %w", zap.Error(err))
			return err
		}
	}

	dr.KRetNfNatInet, err = link.Kretprobe(nfNatInetFn, objs.NfNatInetFnRet, nil)
	if err != nil {
		// TODO: remove this check once we get this working on Mariner OS.
		if errors.Is(err, os.ErrNotExist) {
			dr.l.Warn("nf_nat_inet_fn_ret not found, skipping attaching kretprobe to it. This may impact the drop reason metrics.")
		} else {
			dr.l.Error("opening kretprobe: %w", zap.Error(err))
			return err
		}
	}

	dr.KNfConntrackConfirm, err = link.Kprobe(nfConntrackConfirmFn, objs.NfConntrackConfirm, nil)
	if err != nil {
		// TODO: remove this check once we get this working on Mariner OS.
		if errors.Is(err, os.ErrNotExist) {
			dr.l.Warn("nf_conntrack_confirm not found, skipping attaching kprobe to it. This may impact the drop reason metrics.")
		} else {
			dr.l.Error("opening kprobe: %w", zap.Error(err))
			return err
		}
	}

	dr.KRetNfConntrackConfirm, err = link.Kretprobe(nfConntrackConfirmFn, objs.NfConntrackConfirmRet, nil)
	if err != nil {
		// TODO: remove this check once we get this working on Mariner OS.
		if errors.Is(err, os.ErrNotExist) {
			dr.l.Warn("nf_conntrack_confirm_ret not found, skipping attaching kretprobe to it. This may impact the drop reason metrics.")
		} else {
			dr.l.Error("opening kretprobe: %w", zap.Error(err))
			return err
		}
	}

	dr.metricsMapData = objs.RetinaDropreasonMetrics
	return nil
}

func (dr *dropReason) Start(ctx context.Context) error {
	dr.isRunning = true
	dr.l.Info("Start listening for drop reason events...")

	if dr.cfg.EnablePodLevel {
		// Setup records channel.
		dr.recordsChannel = make(chan perf.Record, buffer)

		dr.l.Info("setting up enricher since pod level is enabled")
		// Set up enricher.
		if enricher.IsInitialized() {
			dr.enricher = enricher.Instance()
		} else {
			dr.l.Warn("retina enricher is not initialized")
		}
	} else {
		dr.l.Info("will not set up enricher since pod level is disabled")
	}

	return dr.run(ctx)
}

func (dr *dropReason) SetupChannel(ch chan *hubblev1.Event) error {
	dr.externalChannel = ch
	return nil
}

// Helper function that accepts an IMap interface and returns an *ebpf.MapIterator
// Added to facilitate unit tests
func convertIMapToIMapIterator(inMap IMap) IMapIterator {
	return inMap.Iterate()
}

// Assign the helper function to a variable, this will make it easier to mock in unit tests
var iMapIterator func(IMap) IMapIterator = convertIMapToIMapIterator

func (dr *dropReason) run(ctx context.Context) error {
	go dr.readBasicMetricsData(ctx)

	if dr.cfg.EnablePodLevel {
		for i := 0; i < workers; i++ {
			dr.wg.Add(1)
			go dr.processRecord(ctx, i)
		}
		// run advanced group here.
		// Don't add it to the wait group because we don't want to wait for it.
		// The perf reader Read call blocks until there is data available in the perf buffer.
		// That call is unblocked when Reader is closed.
		go dr.readAdvancedMetricsData(ctx)
	}

	<-ctx.Done()
	// Wait for all workers to finish.
	// Only relevant when pod level is enabled.
	dr.wg.Wait()

	dr.l.Info("Context is done, stopping DropReason plugin")
	return nil
}

func (dr *dropReason) readBasicMetricsData(ctx context.Context) {
	var dataKey dropMetricKey
	var dataValue dropMetricValues
	ticker := time.NewTicker(dr.cfg.MetricsInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			dr.l.Info("Context is done, dropreason basic metrics loop will stop running")
			return
		case <-ticker.C:
			var iter IMapIterator
			iter = iMapIterator(dr.metricsMapData)
			if err := iter.Err(); err != nil {
				dr.l.Error("Error while reading metrics map...", zap.String("iter error", err.Error()))
			}
			for iter.Next(&dataKey, &dataValue) {
				dr.processMapValue(dataKey, dataValue)
			}

			// TODO manage deletiong of old entries
			// If we start deleting keys, we need to change the metric add logic in DropMetricAdd
		}
	}
}

func (dr *dropReason) readAdvancedMetricsData(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			dr.l.Info("Context is done, dropreason advanced metrics loop will stop running")
			return
		default:
			if err := dr.readEventArrayData(); err != nil {
				dr.l.Error("Error reading event array data", zap.Error(err))
			}
		}
	}
}

func (dr *dropReason) processRecord(ctx context.Context, id int) {
	dr.l.Debug("Starting worker", zap.Int("id", id))
	defer dr.wg.Done()
	for {
		select {
		case <-ctx.Done():
			dr.l.Info("Context is done, dropreason worker will stop running", zap.Int("id", id))
			return
		case record := <-dr.recordsChannel:
			var bpfEvent kprobePacket
			err := binary.Read(bytes.NewReader(record.RawSample), binary.LittleEndian, &bpfEvent)
			if err != nil {
				if binary.Size(bpfEvent) != len(record.RawSample) {
					dr.l.Error("Error reading bpf event due to size mismatch", zap.Error(err), zap.Int("expected", binary.Size(bpfEvent)), zap.Int("actual", len(record.RawSample)))
				} else {
					dr.l.Error("Error reading bpf event", zap.Error(err))
				}
				continue
			}
			sourcePortShort := uint32(utils.HostToNetShort(bpfEvent.SrcPort))
			destinationPortShort := uint32(utils.HostToNetShort(bpfEvent.DstPort))

			fl := utils.ToFlow(
				dr.l,
				ktime.MonotonicOffset.Nanoseconds()+int64(bpfEvent.Ts),
				utils.Int2ip(bpfEvent.SrcIp).To4(), // Precautionary To4() call.
				utils.Int2ip(bpfEvent.DstIp).To4(), // Precautionary To4() call.
				sourcePortShort,
				destinationPortShort,
				bpfEvent.Proto,
				2, // drop reason packet doesn't have a direction yet, so we set it to 2
				flow.Verdict_DROPPED,
			)
			if fl == nil {
				dr.l.Warn("Could not convert bpfEvent to flow", zap.Any("bpfEvent", bpfEvent))
				continue
			}

			// IsReply is not applicable for DROPPED verdicts.
			fl.IsReply = nil

			meta := &utils.RetinaMetadata{}

			// Add drop reason to the flow's metadata.
			utils.AddDropReason(fl, meta, bpfEvent.DropType)

			// Add packet size to the flow's metadata.
			utils.AddPacketSize(meta, bpfEvent.SkbLen)

			// Add metadata to the flow.
			utils.AddRetinaMetadata(fl, meta)

			// This is only for development purposes.
			// Removing this makes logs way too chatter-y.
			dr.l.Debug("DropReason Packet Received", zap.Any("flow", fl), zap.Any("Raw Bpf Event", bpfEvent), zap.Uint16("drop type", bpfEvent.DropType))

			// Write the event to the enricher.
			ev := &hubblev1.Event{
				Event:     fl,
				Timestamp: fl.GetTime(),
			}
			if dr.enricher != nil {
				dr.enricher.Write(ev)
			}

			// Send event to external channel.
			if dr.externalChannel != nil {
				select {
				case dr.externalChannel <- ev:
				default:
					metrics.LostEventsCounter.WithLabelValues(utils.ExternalChannel, string(Name)).Inc()
				}
			}
		}
	}
}

func (dr *dropReason) readEventArrayData() error {
	record, err := dr.reader.Read()
	if err != nil {
		if errors.Is(err, perf.ErrClosed) {
			dr.l.Warn("Perf array is empty")
			// nothing to do, we're done
			return nil
		} else {
			dr.l.Error("Error reading perf array", zap.Error(err))
			return fmt.Errorf("Error reading perf array")
		}
	}

	if record.LostSamples > 0 {
		// dr.l.Warn("Lost samples in drop reason plugin", zap.Uint64("lost samples", record.LostSamples))
		metrics.LostEventsCounter.WithLabelValues(utils.Kernel, string(Name)).Add(float64(record.LostSamples))
		return nil
	}

	select {
	case dr.recordsChannel <- record:
		dr.l.Debug("Record sent to channel", zap.Any("record", record))
	default:
		// dr.l.Warn("Channel is full, dropping record", zap.Any("record", record))
		metrics.LostEventsCounter.WithLabelValues(utils.BufferedChannel, string(Name)).Inc()
	}

	return nil
}

func (dr *dropReason) processMapValue(dataKey dropMetricKey, dataValue dropMetricValues) {
	pktCount, pktBytes := dataValue.getPktCountAndBytes()

	dr.l.Debug("DATA From the DropReason Map", zap.String("Droptype", dataKey.getType()),
		zap.Uint32("Return Val", dataKey.ReturnVal),
		zap.Int("DropCount", int(pktCount)),
		zap.Int("DropBytes", int(pktBytes)))

	dr.dropMetricAdd(dataKey.getType(), dataKey.getDirection(), pktCount, pktBytes)
}

func (dr *dropReason) Stop() error {
	if !dr.isRunning {
		return nil
	}
	dr.l.Info("Closing drop reason probes...")
	if dr.KNfHook != nil {
		dr.KNfHook.Close()
	}
	if dr.KRetnfhook != nil {
		dr.KRetnfhook.Close()
	}
	if dr.KRetTCPConnect != nil {
		dr.KRetTCPConnect.Close()
	}
	if dr.KRetTCPAccept != nil {
		dr.KRetTCPAccept.Close()
	}
	if dr.KTCPAccept != nil {
		dr.KTCPAccept.Close()
	}
	if dr.metricsMapData != nil {
		dr.metricsMapData.Close()
	}

	if dr.reader != nil {
		if err := dr.reader.Close(); err != nil {
			dr.l.Error("Error closing perf reader", zap.Error(err))
		}
	}

	// Close records channel.
	// At this point, all workers should have exited,
	// as well as the producer of the records channel.
	if dr.recordsChannel != nil {
		close(dr.recordsChannel)
		dr.l.Debug("Closed records channel")
	}

	dr.l.Info("Exiting DropReason Stop...")
	dr.isRunning = false
	return nil
}

func (dr *dropReason) dropMetricAdd(reason string, direction string, count float64, bytes float64) {
	metrics.DropPacketsGauge.WithLabelValues(reason, direction).Set(float64(count))
	metrics.DropBytesGauge.WithLabelValues(reason, direction).Set(float64(bytes))
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
