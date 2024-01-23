// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
//go:build exclude

package tcpreceive

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
	Name         api.PluginName = "tcpreceive"
	TCPBytesRecv string         = "tcp_bytes_recv"
	Proto        string         = "proto"
	counterName  string         = "tcprecv_count"
)

//nolint:unused
type tcpreceive struct {
	l           *log.ZapLogger
	kRecv       link.Link
	hashmapData *ebpf.Map
	event       chan<- api.PluginEvent
}

type TcpreceiveData struct {
	LocalIP    net.IP
	RemoteIP   net.IP
	LocalPort  uint16
	RemotePort uint16
	L4Proto    string
	Recv       int32
	Op         string
}

func (r *TcpreceiveData) GetPluginEventAttributes(pluginname string, c *cache.Cache) []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, 0)
	attrs = utils.GetPluginEventAttributes(attrs, pluginname, r.Op, time.Now().String())
	attrs = utils.GetPluginEventLocalAttributes(attrs, c, r.LocalIP.String(), fmt.Sprint(r.LocalPort))
	attrs = utils.GetPluginEventRemoteAttributes(attrs, c, r.RemoteIP.String(), fmt.Sprint(r.RemotePort))
	attrs = append(attrs,
		attribute.Key(TCPBytesRecv).Int(int(r.Recv)),
		attribute.Key(Proto).String(r.L4Proto),
	)
	return attrs
}

func (r *TcpreceiveData) HandlePluginEventSignals(attr []attribute.KeyValue, m metric.Meter, t trace.Tracer) {
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

func (t TcpreceiveData) String() string {
	return fmt.Sprintf("LocalAddr:%s LocalPort:%d RemoteAddr:%s RemotePort:%d L4Proto:%s Operation:%s Recv:%d",
		t.LocalIP.String(), t.LocalPort, t.RemoteIP.String(), t.RemotePort, t.L4Proto, t.Op, t.Recv)
}
