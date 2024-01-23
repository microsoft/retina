// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
//go:build exclude

package iptablelogger

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/microsoft/retina/pkg/deprecated/cache"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/plugin/api"
	"github.com/microsoft/retina/pkg/utils"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const (
	Name        api.PluginName = "iptablelogger"
	counterName string         = "iptablelogger_count"
)

//nolint:unused
type iptableLogger struct {
	l          *log.ZapLogger
	Knfhook    link.Link
	Kretnfhook link.Link
	verdictRd  *perf.Reader
	event      chan<- api.PluginEvent
}

type Chain uint32

type IPTableEvent struct {
	Timestamp   uint64
	Pid         uint32
	ProcessName string
	SrcIP       net.IP
	DstIP       net.IP
	SrcPort     uint16
	DstPort     uint16
	Hook        Chain
	Verdict     int32
}

func (ipe *IPTableEvent) HandlePluginEventSignals(attr []attribute.KeyValue, m metric.Meter, t trace.Tracer) {
	// metric
	cnter, err := m.Int64Counter(counterName)
	if err == nil {
		cnter.Add(context.TODO(), 1, attr...)
	}

	// trace
	_, span := t.Start(context.Background(), string(Name))
	span.SetAttributes(attr...)
	span.End()
}

func (ipe *IPTableEvent) GetPluginEventAttributes(pluginname string, c *cache.Cache) []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, 0)
	attrs = utils.GetPluginEventAttributes(attrs, pluginname, ipe.Hook.String(), time.Now().String())
	attrs = utils.GetPluginEventLocalAttributes(attrs, c, ipe.SrcIP.String(), fmt.Sprint(ipe.SrcPort))
	attrs = utils.GetPluginEventRemoteAttributes(attrs, c, ipe.DstIP.String(), fmt.Sprint(ipe.DstPort))
	return attrs
}

const (
	INPUT Chain = iota
	PREROUTING
	FORWARD
	OUTPUT
	POSTROUTING
	UNKNOWN
)

func (c Chain) String() string {
	switch c {
	case INPUT:
		return "INPUT"
	case PREROUTING:
		return "PREROUTING"
	case FORWARD:
		return "FORWARD"
	case OUTPUT:
		return "OUTPUT"
	case POSTROUTING:
		return "POSTROUTING"
	default:
		return "UNKNOWN"
	}
}

func (t IPTableEvent) String() string {
	return fmt.Sprintf("Timestamp:%d Pid:%d ProcessName:%s Hook:%s SrcAddr:%s SrcPort:%d DstAddr:%s  DstPort:%d Verdict:%d",
		t.Timestamp, t.Pid, t.ProcessName, t.Hook.String(), t.SrcIP.String(), t.SrcPort, t.DstIP.String(), t.DstPort, t.Verdict)
}
