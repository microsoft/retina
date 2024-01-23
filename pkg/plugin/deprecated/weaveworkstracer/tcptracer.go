// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
//go:build exclude

package tcptracer

import (
	"context"

	"github.com/microsoft/retina/pkg/plugin/api"
	"github.com/weaveworks/tcptracer-bpf/pkg/tracer"
	"go.uber.org/zap"
)

// TCPEventV4 handle tcpv4 packets
func (t *tracerInfo) TCPEventV4(e tracer.TcpV4) {
	if e.Type == tracer.EventFdInstall {
		return
	}

	tcpEvent := &TcpData{
		SrcAddr:     e.SAddr,
		DstAddr:     e.DAddr,
		SrcPort:     e.SPort,
		DstPort:     e.DPort,
		ProcessName: e.Comm,
		ProcessId:   e.Pid,
		Op:          Operation(tracer.EventConnect.String()),
	}
	pluginEvent := api.PluginEvent{
		Name:  Name,
		Event: tcpEvent,
	}
	// notify event channel about packet
	t.Event <- pluginEvent
}

func (fl *tracerInfo) TCPEventV6(e tracer.TcpV6) {
}

func (fl *tracerInfo) LostV4(count uint64) {
}

func (fl *tracerInfo) LostV6(count uint64) {
}

// NewTcpTracerPlugin Initialize logger object
func New(logger *zap.Logger) api.Plugin {
	return &TcpTracerPlugin{
		l: logger,
	}
}

// Init intialize Tcp Tracer
func (ttp *TcpTracerPlugin) Init(pluginEvent chan<- api.PluginEvent) error {
	var err error
	ttp.l.Info("Initializing tcptracer plugin ...")
	ttp.tracer = &tracerInfo{
		l: ttp.l,
	}
	ttp.tracer.t, err = tracer.NewTracer(ttp.tracer)
	if err != nil {
		ttp.l.Error("Failed to initialize tcp tracer:%w", zap.Error(err))
		return err
	}
	ttp.tracer.Event = pluginEvent
	return nil
}

func (ttp *TcpTracerPlugin) Start(ctx context.Context) error {
	ttp.l.Info("Starting tcptracer plugin ...")
	ttp.tracer.t.Start()
	return nil
}

func (ttp *TcpTracerPlugin) Stop() error {
	ttp.l.Info("Stopping tcptracer plugin ...")
	if ttp.tracer.t != nil {
		ttp.tracer.t.Stop()
	}
	return nil
}

func NewPluginFn(l *zap.Logger) api.Plugin {
	return New(l)
}
