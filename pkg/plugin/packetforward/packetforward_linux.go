// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

// package packetforward contains the Retina packetforward plugin. It utilizes eBPF to measures
// packets and bytes passing through the eth0 interface of each node, along with the direction of the packets.
package packetforward

import (
	"context"
	"fmt"
	"path"
	"runtime"
	"syscall"
	"time"

	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/pkg/errors"

	hubblev1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	"github.com/cilium/ebpf"
	"github.com/microsoft/retina/pkg/loader"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/metrics"
	"github.com/microsoft/retina/pkg/plugin/api"
	"github.com/microsoft/retina/pkg/utils"
	"go.uber.org/zap"

	_ "github.com/microsoft/retina/pkg/plugin/packetforward/_cprog" // nolint
)

//go:generate bpf2go -cflags "-g -O2 -Wall -D__TARGET_ARCH_${GOARCH} -Wall" -target ${GOARCH} -type metric packetforward ./_cprog/packetforward.c -- -I../lib/_${GOARCH} -I../lib/common/libbpf/_src

// New creates a new packetforward plugin.
func New(cfg *kcfg.Config) api.Plugin {
	return &packetForward{
		cfg: cfg,
		l:   log.Logger().Named(string(Name)),
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

func processMapValue(m IMap, key uint32) (uint64, uint64, error) {
	/* Sample of values in hashmap m.
	"values": [{
				"cpu": 0,
				"value": {
					"count": 2292,
					"bytes": 202689
				}
				},{
					"cpu": 1,
					"value": {
						"count": 1053,
						"bytes": 89579
					}
				}]
	*/
	var totalCount, totalBytes uint64
	var err error
	values := []packetforwardMetric{} //nolint:typecheck
	if err = m.Lookup(key, &values); err == nil {
		for _, v := range values {
			totalCount = totalCount + v.Count
			totalBytes = totalBytes + v.Bytes
		}
	}
	return totalCount, totalBytes, err
}

func updateMetrics(data *PacketForwardData) {
	// Add the packet count metrics.
	metrics.ForwardPacketsGauge.WithLabelValues(ingressLabel).Set(float64(data.ingressCountTotal))
	metrics.ForwardPacketsGauge.WithLabelValues(egressLabel).Set(float64(data.egressCountTotal))

	// Add the packet bytes metrics.
	metrics.ForwardBytesGauge.WithLabelValues(ingressLabel).Set(float64(data.ingressBytesTotal))
	metrics.ForwardBytesGauge.WithLabelValues(egressLabel).Set(float64(data.egressBytesTotal))
}

// Plugin API implementation for packet forward.
// Ref: github.com/microsoft/retina/pkg/plugin/api

func (p *packetForward) Name() string {
	return string(Name)
}

func (p *packetForward) Generate(ctx context.Context) error {
	// Use this function to parse p and generate header files under cprog.
	// Example: https://github.com/anubhabMajumdar/Retina/blob/c4bc06e7f922124f92536ffb5312bada5c2dfe99/pkg/plugin/custom/packetforward/packetforward.go#L77
	p.l.Info("Packet forwarding metric header generated")
	return nil
}

func (p *packetForward) Compile(ctx context.Context) error {
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
	// Keep target as bpf, otherwise clang compilation yields bpf object that elf reader cannot load.
	err = loader.CompileEbpf(ctx, "-target", "bpf", "-Wall", targetArch, "-g", "-O2", "-c", bpfSourceFile, "-o", bpfOutputFile, includeDir, libbpfDir)
	if err != nil {
		return errors.Wrap(err, "error compiling ebpf code")
	}
	p.l.Info("Packet forwarding metric compiled")
	return nil
}

func (p *packetForward) Init() error {
	// Get the absolute path to this file during runtime.
	dir, err := absPath()
	if err != nil {
		return err
	}

	bpfOutputFile := fmt.Sprintf("%s/%s", dir, bpfObjectFileName)

	objs := &packetforwardObjects{} //nolint:typecheck
	spec, err := ebpf.LoadCollectionSpec(bpfOutputFile)
	if err != nil {
		p.l.Error("Error loading collection specs: %w", zap.Error(err))
		return err
	}

	if err := spec.LoadAndAssign(objs, nil); err != nil {
		p.l.Error("Error assigning specs: %w", zap.Error(err))
		return err
	}

	p.hashmapData = objs.RetinaPacketforwardMetrics

	// The steps to attach ebpf to socket is documented in cilium/ebpf
	// https://github.com/cilium/ebpf/blob/master/example_sock_elf_test.go#L85.
	// MIT license.
	p.sock, err = utils.OpenRawSocket(socketIndex)
	if err != nil {
		p.l.Error("Error opening socket %d: %w", zap.Int("eth", socketIndex), zap.Error(err))
		return err
	}

	if err := syscall.SetsockoptInt(p.sock, syscall.SOL_SOCKET, PacketForwardSocketAttach, objs.SocketFilter.FD()); err != nil {
		p.l.Error("Error attaching packet forward socket filter: %w", zap.Error(err))
		return err
	}

	p.l.Info("Packet forwarding metric initialized")
	return nil
}

func (p *packetForward) Start(ctx context.Context) error {
	p.l.Info("Start collecting packet forward metrics")
	p.isRunning = true
	return p.run(ctx)
}

func (p *packetForward) Stop() error {
	if !p.isRunning {
		return nil
	}
	if p.hashmapData != nil {
		p.hashmapData.Close()
	}
	if p.sock != 0 {
		syscall.Close(p.sock)
	}
	p.l.Info("Exiting forwarding metrics")
	p.isRunning = false
	return nil
}

func (p *packetForward) SetupChannel(ch chan *hubblev1.Event) error {
	p.l.Debug("SetupChannel is not supported by plugin", zap.String("plugin", string(Name)))
	return nil
}

func (p *packetForward) run(ctx context.Context) error {
	var err error
	ticker := time.NewTicker(p.cfg.MetricsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.l.Info("Context is done, packetforward will stop running")
			return nil
		case <-ticker.C:
			data := &PacketForwardData{}
			data.ingressCountTotal, data.ingressBytesTotal, err = processMapValue(p.hashmapData, ingressKey)
			if err != nil {
				p.l.Error("Error reading hash map", zap.Error(err))
				continue
			}
			data.egressCountTotal, data.egressBytesTotal, err = processMapValue(p.hashmapData, egressKey)
			if err != nil {
				p.l.Error("Error reading hash map", zap.Error(err))
				continue
			}
			p.l.Debug("Received PacketForward data", zap.String("Data", data.String()))
			updateMetrics(data)
		}
	}
}
