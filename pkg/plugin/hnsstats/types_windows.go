// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package hnsstats

import (
	"context"
	"fmt"
	"time"

	"github.com/Microsoft/hcsshim"
	"github.com/Microsoft/hcsshim/hcn"
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/log"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"go.opentelemetry.io/otel/trace"
)

const (
	name          string = "hnsstats"
	HnsStatsEvent string = "hnsstatscount"

	// Advanced metric
	AdvHNSStatsName        string = "adv_windows_hns_stats"
	AdvHNSStatsDescription string = "Include many different metrics from packets sent/received to closed connections"

	// Advanced metric labels
	Ip           string = "ip"
	Port         string = "port"
	Namespace    string = "namespace"
	PodName      string = "podname"
	WorkloadKind string = "workload_kind"
	WorkloadName string = "workload_name"

	// From HNSStats API
	PacketsReceived        string = "win_packets_recv_count"
	PacketsSent            string = "win_packets_sent_count"
	BytesSent              string = "win_bytes_sent_count"
	BytesReceived          string = "win_bytes_recv_count"
	DroppedPacketsIncoming string = "win_packets_recv_drop_count"
	DroppedPacketsOutgoing string = "win_packets_sent_drop_count"
	DummyPort              string = "80" // Required arg in GetLocalEventAttributes
	// From VFP TCP counters
	// IN:
	// VFP TCP Packet Counters IN
	SynPacketCountIn    string = "win_tcp_recv_syn_packet_count"
	SynAckPacketCountIn string = "win_tcp_recv_syn_ack_packet_count"
	FinPacketCountIn    string = "win_tcp_recv_fin_packet_count"
	RstPacketCountIn    string = "win_tcp_recv_rst_packet_count"
	// VFP DROP Counters IN
	AclDropPacketCountIn string = "win_acl_recv_drop_packet_count"
	// VFP TCP Connections Counters IN
	VerifiedCountIn            string = "win_tcp_recv_verified_count"
	TimedOutCountIn            string = "win_tcp_recv_timedout_count"
	ResetCountIn               string = "win_tcp_recv_rst_count"
	ResetSynCountIn            string = "win_tcp_recv_rst_syn_count"
	ClosedFinCountIn           string = "win_tcp_recv_closed_fin_count"
	TcpHalfOpenTimeoutsCountIn string = "win_tcp_recv_half_open_timeout_count"
	TimeWaitExpiredCountIn     string = "win_tcp_recv_time_wait_expired_count"
	// OUT:
	// VFP TCP Packet Counters OUT
	SynPacketCountOut    string = "win_tcp_sent_syn_packet_count"
	SynAckPacketCountOut string = "win_tcp_sent_syn_ack_packet_count"
	FinPacketCountOut    string = "win_tcp_sent_fin_packet_count"
	RstPacketCountOut    string = "win_tcp_sent_rst_packet_count"
	// VFP DROP Counters Out
	AclDropPacketCountOut string = "win_acl_sent_drop_packet_count"
	// VFP TCP Connection Counters OUT
	VerifiedCountOut            string = "win_tcp_sent_verified_count"
	TimedOutCountOut            string = "win_tcp_sent_timedout_count"
	ResetCountOut               string = "win_tcp_sent_rst_count"
	ResetSynCountOut            string = "win_tcp_sent_rst_syn_count"
	ClosedFinCountOut           string = "win_tcp_sent_closed_fin_count"
	TcpHalfOpenTimeoutsCountOut string = "win_tcp_sent_half_open_timeout_count"
	TimeWaitExpiredCountOut     string = "win_tcp_sent_time_wait_expired_count"

	// metrics direction
	ingressLabel = "ingress"
	egressLabel  = "egress"
)

type hnsstats struct {
	cfg           *kcfg.Config
	interval      time.Duration
	state         int
	l             *log.ZapLogger
	endpointQuery hcn.HostComputeQuery
	enricher      enricher.EnricherInterface
}

type HnsStatsData struct {
	hnscounters *hcsshim.HNSEndpointStats
	IPAddress   string
	vfpCounters *VfpPortStatsData
	Port        string
}

// handles event signals such as incrementing a metric counter
func (h *HnsStatsData) HandlePluginEventSignals(attr []attribute.KeyValue, m metric.Meter, t trace.Tracer) {
	h.addHnsStatsEventCounters(&attr, m)
	h.vfpCounters.addVfpStatsEventCounters(&attr, m)
}

// not used at the moment, but persisting until everything is ported to metrics.*
// Adds HNS endpoint stats counters
func (h *HnsStatsData) addHnsStatsEventCounters(attr *[]attribute.KeyValue, m metric.Meter) {
	updateCounter(PacketsReceived, attr, m, int64(h.hnscounters.PacketsReceived))
	updateCounter(PacketsSent, attr, m, int64(h.hnscounters.PacketsSent))
	updateCounter(BytesSent, attr, m, int64(h.hnscounters.BytesSent))
	updateCounter(BytesReceived, attr, m, int64(h.hnscounters.BytesReceived))
	updateCounter(DroppedPacketsIncoming, attr, m, int64(h.hnscounters.DroppedPacketsIncoming))
	updateCounter(DroppedPacketsOutgoing, attr, m, int64(h.hnscounters.DroppedPacketsOutgoing))
}

// not used at the moment, but persisting until everything is ported to metrics.*
// Adds VFP stats counters
func (v *VfpPortStatsData) addVfpStatsEventCounters(attr *[]attribute.KeyValue, m metric.Meter) {
	// IN TCP Packet counters
	updateCounter(SynPacketCountIn, attr, m, int64(v.In.TcpCounters.PacketCounters.SynPacketCount))
	updateCounter(SynAckPacketCountIn, attr, m, int64(v.In.TcpCounters.PacketCounters.SynAckPacketCount))
	updateCounter(FinPacketCountIn, attr, m, int64(v.In.TcpCounters.PacketCounters.FinPacketCount))
	updateCounter(RstPacketCountIn, attr, m, int64(v.In.TcpCounters.PacketCounters.RstPacketCount))
	// IN DROP counters:
	updateCounter(AclDropPacketCountIn, attr, m, int64(v.In.DropCounters.AclDropPacketCount))
	// IN TCP Connection Counters
	updateCounter(VerifiedCountIn, attr, m, int64(v.In.TcpCounters.ConnectionCounters.VerifiedCount))
	updateCounter(TimedOutCountIn, attr, m, int64(v.In.TcpCounters.ConnectionCounters.TimedOutCount))
	updateCounter(ResetCountIn, attr, m, int64(v.In.TcpCounters.ConnectionCounters.ResetCount))
	updateCounter(ResetSynCountIn, attr, m, int64(v.In.TcpCounters.ConnectionCounters.ResetSynCount))
	updateCounter(ClosedFinCountIn, attr, m, int64(v.In.TcpCounters.ConnectionCounters.ClosedFinCount))
	updateCounter(TcpHalfOpenTimeoutsCountIn, attr, m, int64(v.In.TcpCounters.ConnectionCounters.TcpHalfOpenTimeoutsCount))
	updateCounter(TimeWaitExpiredCountIn, attr, m, int64(v.In.TcpCounters.ConnectionCounters.TimeWaitExpiredCount))
	// OUT TCP Packet counters
	updateCounter(SynPacketCountOut, attr, m, int64(v.Out.TcpCounters.PacketCounters.SynPacketCount))
	updateCounter(SynAckPacketCountOut, attr, m, int64(v.Out.TcpCounters.PacketCounters.SynAckPacketCount))
	updateCounter(FinPacketCountOut, attr, m, int64(v.Out.TcpCounters.PacketCounters.FinPacketCount))
	updateCounter(RstPacketCountOut, attr, m, int64(v.Out.TcpCounters.PacketCounters.RstPacketCount))
	// OUT DROP counters:
	updateCounter(AclDropPacketCountOut, attr, m, int64(v.Out.DropCounters.AclDropPacketCount))
	// OUT TCP Connection Counters
	updateCounter(VerifiedCountOut, attr, m, int64(v.Out.TcpCounters.ConnectionCounters.VerifiedCount))
	updateCounter(TimedOutCountOut, attr, m, int64(v.Out.TcpCounters.ConnectionCounters.TimedOutCount))
	updateCounter(ResetCountOut, attr, m, int64(v.Out.TcpCounters.ConnectionCounters.ResetCount))
	updateCounter(ResetSynCountOut, attr, m, int64(v.Out.TcpCounters.ConnectionCounters.ResetSynCount))
	updateCounter(ClosedFinCountOut, attr, m, int64(v.Out.TcpCounters.ConnectionCounters.ClosedFinCount))
	updateCounter(TcpHalfOpenTimeoutsCountOut, attr, m, int64(v.Out.TcpCounters.ConnectionCounters.TcpHalfOpenTimeoutsCount))
	updateCounter(TimeWaitExpiredCountOut, attr, m, int64(v.Out.TcpCounters.ConnectionCounters.TimeWaitExpiredCount))
}

func updateCounter(counterName string, attr *[]attribute.KeyValue, m metric.Meter, count int64) {
	// metric
	cnter, err := m.Int64Counter(counterName)
	// Convert the attributes to metric AddOption.
	opt := metric.WithAttributes(*attr...)
	if err == nil {
		cnter.Add(context.TODO(), count, opt)
	}
}

func (h *HnsStatsData) String() string {
	return fmt.Sprintf("Endpoint ID: %s, Packets received: %d, Packets sent %d, Bytes sent %d, Bytes received %d",
		h.hnscounters.EndpointID, h.hnscounters.PacketsReceived, h.hnscounters.PacketsSent, h.hnscounters.BytesSent, h.hnscounters.BytesReceived)
}
