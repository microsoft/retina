// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
//go:build exclude

package tcpsend

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/microsoft/retina/pkg/deprecated/cache"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/plugin/api"
	"github.com/microsoft/retina/pkg/utils"
	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const (
	Name         api.PluginName = "tcpsend"
	TCPBytesSend string         = "tcp_bytes_send"
	Proto        string         = "proto"
	counterName  string         = "tcpsend_count"
)

//nolint:unused
type tcpsend struct {
	l           *log.ZapLogger
	kSend       link.Link
	hashmapData *ebpf.Map
	event       chan<- api.PluginEvent
}

type TcpsendData struct {
	LocalIP    net.IP
	RemoteIP   net.IP
	LocalPort  uint16
	RemotePort uint16
	L4Proto    string
	Sent       uint64
	Op         string
}

func (s *TcpsendData) GetPluginEventAttributes(pluginname string, c *cache.Cache) []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, 0)
	attrs = utils.GetPluginEventAttributes(attrs, pluginname, s.Op, time.Now().String())
	attrs = utils.GetPluginEventLocalAttributes(attrs, c, s.LocalIP.String(), fmt.Sprint(s.LocalPort))
	attrs = utils.GetPluginEventRemoteAttributes(attrs, c, s.RemoteIP.String(), fmt.Sprint(s.RemotePort))
	attrs = append(attrs,
		attribute.Key(TCPBytesSend).Int(int(s.Sent)),
		attribute.Key(Proto).String(s.L4Proto),
	)
	return attrs
}

func (s *TcpsendData) HandlePluginEventSignals(attr []attribute.KeyValue, m metric.Meter, t trace.Tracer) {
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

func (t TcpsendData) String() string {
	return fmt.Sprintf("LocalAddr:%s LocalPort:%d RemoteAddr:%s RemotePort:%d L4Proto:%s Operation:%s Sent:%d",
		t.LocalIP.String(), t.LocalPort, t.RemoteIP.String(), t.RemotePort, t.L4Proto, t.Op, t.Sent)
}
