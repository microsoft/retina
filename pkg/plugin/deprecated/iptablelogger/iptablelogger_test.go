// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
//go:build exclude

package iptablelogger

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

func TestIPTableLoggerEvent(t *testing.T) {
	l := log.SetupZapLogger(log.GetDefaultLogOpts())
	tt := &iptableLogger{l: l}
	srcIP := "192.168.0.4"
	dstIP := "192.168.0.5"
	flow := kprobeIptuple{
		Saddr: ip2int(net.ParseIP(srcIP)),
		Daddr: ip2int(net.ParseIP(dstIP)),
		Sport: 1234,
		Dport: 2345,
		Proto: 6,
	}
	e := kprobeVerdict{
		Flow:   flow,
		Ts:     1234,
		Status: 1,
		Pid:    uint32(1234),
	}

	event := make(chan api.PluginEvent, 500)
	tt.event = event
	notifyVerdictData(tt, e)

	var data api.PluginEvent
	select {
	case data = <-event:
		var ipt *IPTableEvent
		ipt = data.Event.(*IPTableEvent)
		require.Exactly(t, Name, data.Name, "Expected:%s but got:%s", Name, data.Name)
		require.Equal(t, srcIP, ipt.SrcIP.String(), "Expected:%s but got:%s", srcIP, ipt.SrcIP.String())
		require.Equal(t, e.Flow.Sport, ipt.SrcPort, "Expected:%d but got:%d", e.Flow.Sport, ipt.SrcPort)
		require.Equal(t, dstIP, ipt.DstIP.String(), "Expected:%s but got:%s", dstIP, ipt.DstIP.String())
		require.Equal(t, e.Flow.Dport, ipt.DstPort, "Expected:%d but got:%d", e.Flow.Dport, ipt.DstPort)
		require.Equal(t, e.Pid, ipt.Pid, "Expected:%d but got:%d", e.Pid, ipt.Pid)
		require.Equal(t, e.Ts, ipt.Timestamp, "Expected:%d but got:%d", e.Ts, ipt.Timestamp)
		require.Equal(t, e.Status, ipt.Verdict, "Expected:%d but got:%d", e.Status, ipt.Verdict)
	case <-time.After(time.Second * 3):
		t.Fatal("Timedout. Event not received")
	}
}
