// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
// This file is intended to define tcptracerplugin structure and interface
package tcptracer

import (
	"fmt"
	"net"
	"time"

	"github.com/microsoft/retina/pkg/deprecated/cache"
	"github.com/microsoft/retina/pkg/plugin/api"
	"github.com/microsoft/retina/pkg/utils"
	"github.com/weaveworks/tcptracer-bpf/pkg/tracer"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const (
	Name api.PluginName = "tcptracer"
)

type Operation string

type TcpData struct {
	ProcessName string
	ProcessId   uint32
	SrcAddr     net.IP
	DstAddr     net.IP
	SrcPort     uint16
	DstPort     uint16
	Op          Operation
}

func (t *TcpData) GetPluginEventAttributes(pluginname string, c *cache.Cache) []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, 0)
	attrs = utils.GetPluginEventAttributes(attrs, pluginname, string(t.Op), time.Now().String())
	attrs = utils.GetPluginEventLocalAttributes(attrs, c, t.SrcAddr.String(), fmt.Sprint(t.SrcPort))
	attrs = utils.GetPluginEventRemoteAttributes(attrs, c, t.DstAddr.String(), fmt.Sprint(t.DstPort))
	return attrs
}

func (t *TcpData) HandlePluginEventSignals(attr []attribute.KeyValue, m metric.Meter, tr trace.Tracer) {
}

func (t TcpData) String() string {
	return fmt.Sprintf("ProcessName:%s Pid:%d SrcAddr:%s SrcPort:%d DstAddr:%s  DstPort:%d Operaiton:%s",
		t.ProcessName, t.ProcessId, t.SrcAddr, t.SrcPort, t.DstAddr, t.DstPort, t.Op)
}

//nolint:unused
type TcpTracerPlugin struct {
	l      *zap.Logger
	tracer *tracerInfo
}

//nolint:unused
type tracerInfo struct {
	l *zap.Logger
	// weaveworks tcp tracer
	t *tracer.Tracer
	// Plugin use this channel to send data to manager
	Event chan<- api.PluginEvent
}
