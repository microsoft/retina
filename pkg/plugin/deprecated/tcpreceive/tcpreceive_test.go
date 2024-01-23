// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build exclude

package tcpreceive

import (
	"net"
	"os"
	"testing"
	"time"

	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/plugin/api"
	"github.com/stretchr/testify/require"
)

func TestTcpsend(t *testing.T) {
	l := log.SetupZapLogger(log.GetDefaultLogOpts())
	tt := &tcpreceive{l: l}
	srcIP := net.IPv4(192, 168, 0, 4)
	dstIP := net.IPv4(192, 168, 0, 5)

	e := &TcpreceiveData{
		LocalIP:    net.IP(srcIP),
		RemoteIP:   net.IP(dstIP),
		LocalPort:  1234,
		RemotePort: 3446,
		L4Proto:    "TCP",
		Recv:       456,
	}

	event := make(chan api.PluginEvent, 500)
	tt.event = event
	notifyRecvData(tt, e)

	var data api.PluginEvent
	select {
	case data = <-event:
		var tcp *TcpreceiveData
		tcp = data.Event.(*TcpreceiveData)
		require.Exactly(t, Name, data.Name, "Expected:%s but got:%s", Name, data.Name)
		require.Equal(t, srcIP, tcp.LocalIP, "Expected:%s but got:%s", srcIP, tcp.LocalIP)
		require.Equal(t, e.LocalPort, tcp.LocalPort, "Expected:%d but got:%d", e.LocalPort, tcp.LocalPort)
		require.Equal(t, dstIP, tcp.RemoteIP, "Expected:%s but got:%s", dstIP, tcp.RemoteIP)
		require.Equal(t, e.RemotePort, tcp.RemotePort, "Expected:%d but got:%d", e.RemotePort, tcp.RemotePort)
		require.Equal(t, e.L4Proto, tcp.L4Proto, "Expected:%d but got:%d", e.L4Proto, tcp.L4Proto)
		require.Equal(t, e.Recv, tcp.Recv, "Expected:%d but got:%d", e.Recv, tcp.Recv)

	case <-time.After(time.Second * 3):
		t.Fatal("Timedout. Event not received")
	}
}
