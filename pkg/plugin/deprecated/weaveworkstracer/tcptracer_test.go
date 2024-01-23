// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
//go:build exclude

package tcptracer

import (
	"net"
	"testing"
	"time"

	"github.com/microsoft/retina/pkg/plugin/api"
	"github.com/stretchr/testify/require"
	"github.com/weaveworks/tcptracer-bpf/pkg/tracer"
	"go.uber.org/zap"
)

func TestTcpTracerV4Event(t *testing.T) {
	l, _ := zap.NewProduction()
	tt := &tracerInfo{l: l}
	e := tracer.TcpV4{
		Timestamp: 1234,
		Type:      tracer.EventConnect,
		SAddr:     net.ParseIP("192.168.1.4"),
		DAddr:     net.ParseIP("192.168.1.6"),
		SPort:     uint16(2345),
		DPort:     uint16(4567),
		Pid:       uint32(1234),
		Comm:      "UnitTest",
	}
	event := make(chan api.PluginEvent, 500)
	tt.Event = event
	tt.TCPEventV4(e)

	var data api.PluginEvent
	select {
	case data = <-event:
		var tcpdata *TcpData
		tcpdata = data.Event.(*TcpData)
		require.Exactly(t, Name, data.Name, "Expected:%s but got:%s", Name, data.Name)
		require.EqualValues(t, e.Type.String(), tcpdata.Op, "Expected:%s but got:%s", e.Type.String(), tcpdata.Op)
		require.Equal(t, e.SAddr.String(), tcpdata.SrcAddr.String(), "Expected:%s but got:%s", e.SAddr.String(), tcpdata.SrcAddr.String())
		require.Equal(t, e.SPort, tcpdata.SrcPort, "Expected:%d but got:%d", e.SPort, tcpdata.SrcPort)
		require.Equal(t, e.DAddr.String(), tcpdata.DstAddr.String(), "Expected:%s but got:%s", e.DAddr.String(), tcpdata.DstAddr.String())
		require.Equal(t, e.DPort, tcpdata.DstPort, "Expected:%d but got:%d", e.DPort, tcpdata.DstPort)
		require.Equal(t, e.Pid, tcpdata.ProcessId, "Expected:%d but got:%d", e.Pid, tcpdata.ProcessId)
		require.Equal(t, e.Comm, tcpdata.ProcessName, "Expected:%s but got:%s", e.Comm, tcpdata.ProcessName)
	case <-time.After(time.Second * 3):
		t.Fatal("Timedout. Event not received")
	}
}
