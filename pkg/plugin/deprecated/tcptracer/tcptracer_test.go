// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
//go:build exclude

package tcptracer

import (
	"encoding/binary"
	"net"
	"os"
	"testing"
	"time"

	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/plugin/api"
	"github.com/stretchr/testify/require"
)

func ip2int(ip net.IP) uint32 {
	if len(ip) == 16 {
		return binary.LittleEndian.Uint32(ip[12:16])
	}
	return binary.LittleEndian.Uint32(ip)
}

func TestTcpTracer(t *testing.T) {
	l := log.SetupZapLogger(log.GetDefaultLogOpts())
	tt := &tcpTracer{l: l}
	srcIP := "192.168.0.4"
	dstIP := "192.168.0.5"
	e := kprobeTcpv4event{
		Pid:       123,
		Saddr:     ip2int(net.ParseIP(srcIP)),
		Daddr:     ip2int(net.ParseIP(dstIP)),
		Sport:     1234,
		Dport:     3446,
		Operation: uint16(CONNECT),
		SentBytes: 456,
	}

	event := make(chan api.PluginEvent, 500)
	tt.event = event
	notifyTcpV4Data(tt, e)

	var data api.PluginEvent
	select {
	case data = <-event:
		var tcp *TcpV4Data
		tcp = data.Event.(*TcpV4Data)
		require.Exactly(t, Name, data.Name, "Expected:%s but got:%s", Name, data.Name)
		require.Equal(t, e.Operation, uint16(tcp.Op), "Expected:%s but got:%s", Name, data.Name)
		require.Equal(t, srcIP, tcp.LocalIP.String(), "Expected:%s but got:%s", srcIP, tcp.LocalIP.String())
		require.Equal(t, e.Sport, tcp.LocalPort, "Expected:%d but got:%d", e.Sport, tcp.LocalPort)
		require.Equal(t, dstIP, tcp.RemoteIP.String(), "Expected:%s but got:%s", dstIP, tcp.RemoteIP.String())
		require.Equal(t, e.Dport, tcp.RemotePort, "Expected:%d but got:%d", e.Dport, tcp.RemotePort)
		require.Equal(t, e.Pid, tcp.Pid, "Expected:%d but got:%d", e.Pid, tcp.Pid)
		require.Equal(t, e.Ts, tcp.Timestamp, "Expected:%d but got:%d", e.Ts, tcp.Timestamp)
		require.Equal(t, e.SentBytes, tcp.Sent, "Expected:%d but got:%d", e.SentBytes, tcp.Sent)
	case <-time.After(time.Second * 3):
		t.Fatal("Timedout. Event not received")
	}
}
