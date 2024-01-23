// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
//go:build exclude

package tcptracer

import (
	"context"
	"fmt"
	"net"
	"strings"
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
	Name        api.PluginName = "tcptracer"
	Proto       string         = "proto"
	counterName string         = "tcp%s_count"
)

//nolint:unused
type tcpTracer struct {
	l              *log.ZapLogger
	kConnect       link.Link
	kretConnect    link.Link
	kretAccept     link.Link
	kClose         link.Link
	tcpv4Rdconnect *perf.Reader
	tcpv4Rdaccept  *perf.Reader
	tcpv4Rdclose   *perf.Reader
	event          chan<- api.PluginEvent
}

type Operation int

type TcpV4Data struct {
	Timestamp   uint64
	Pid         uint32
	ProcessName string
	LocalIP     net.IP
	RemoteIP    net.IP
	LocalPort   uint16
	RemotePort  uint16
	Sent        uint64
	Recv        int32
	Op          Operation
}

const (
	CONNECT Operation = iota + 1
	ACCEPT
	CLOSE
	SEND
	RECEIVE
)

func (t *TcpV4Data) GetPluginEventAttributes(pluginname string, c *cache.Cache) []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, 0)
	attrs = utils.GetPluginEventAttributes(attrs, pluginname, t.Op.EventString(), time.Now().String())
	attrs = utils.GetPluginEventLocalAttributes(attrs, c, t.LocalIP.String(), fmt.Sprint(t.LocalPort))
	attrs = utils.GetPluginEventRemoteAttributes(attrs, c, t.RemoteIP.String(), fmt.Sprint(t.RemotePort))
	attrs = append(attrs,
		attribute.Key(Proto).String("TCP"),
	)
	return attrs
}

func (t *TcpV4Data) HandlePluginEventSignals(attr []attribute.KeyValue, m metric.Meter, tr trace.Tracer) {
	cnter, err := m.Int64Counter(strings.ToLower(fmt.Sprintf(counterName, t.Op)))
	if err == nil {
		cnter.Add(context.TODO(), 1, attr...)
	}

	// trace
	_, span := tr.Start(context.Background(), string(Name))
	span.SetAttributes(attr...)
	span.End()
}

func (op Operation) String() string {
	switch op {
	case CONNECT:
		return "CONNECT"
	case ACCEPT:
		return "ACCEPT"
	case CLOSE:
		return "CLOSE"
	case SEND:
		return "SEND"
	case RECEIVE:
		return "RECEIVE"
	}
	return "unknown"
}

func (op Operation) EventString() string {
	switch op {
	case CONNECT:
		return "tcpConnectEvent"
	case ACCEPT:
		return "tcpAcceptEvent"
	case CLOSE:
		return "tcpCloseEvent"
	case SEND:
		return "tcpSendEvent"
	case RECEIVE:
		return "tcpRecvEvent"
	}
	return "unknown"
}

func (t TcpV4Data) String() string {
	return fmt.Sprintf("Pid:%d PName: %s LocalAddr:%s LocalPort:%d RemoteAddr:%s  RemotePort:%d Operation:%s Sent:%d Recv:%d",
		t.Pid, t.ProcessName, t.LocalIP.String(), t.LocalPort, t.RemoteIP.String(), t.RemotePort, t.Op.String(), t.Sent, t.Recv)
}
