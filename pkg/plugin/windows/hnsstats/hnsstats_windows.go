// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

// Package hnsstats contains the hnsstats plugin. It gathers TCP statistics and counts number of packets/bytes forwarded or dropped in HNS and VFP from Windows nodes.
package hnsstats

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Microsoft/hcsshim"
	"github.com/Microsoft/hcsshim/hcn"
	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/metrics"
	"github.com/microsoft/retina/pkg/plugin/api"
	"github.com/microsoft/retina/pkg/utils"
	"go.uber.org/zap"
)

const (
	initialize = iota + 1
	start
	stop
)

const (
	HCN_ENDPOINT_STATE_CREATED = iota + 1
	HCN_ENDPOINT_STATE_ATTACHED
	HCN_ENDPOINT_STATE_ATTACHED_SHARING
	HCN_ENDPOINT_STATE_DETACHED
	HCN_ENDPOINT_STATE_DEGRADED
	HCN_ENDPOINT_STATE_DESTROYED
)

func (h *hnsstats) Name() string {
	return string(Name)
}

func (h *hnsstats) Generate(ctx context.Context) error {
	return nil
}

func (h *hnsstats) Compile(ctx context.Context) error {
	return nil
}

func (h *hnsstats) Init() error {
	h.l.Info("Entered hnsstats Init...")
	h.state = initialize
	// Initialize Endpoint query used to filter healthy endpoints (vNIC) of Windows pods
	h.endpointQuery = hcn.HostComputeQuery{
		SchemaVersion: hcn.SchemaVersion{
			Major: 2,
			Minor: 0,
		},
		Flags: hcn.HostComputeQueryFlagsNone,
	}
	// Filter out any endpoints that are not in "AttachedShared" State. All running Windows pods with networking must be in this state.
	filterMap := map[string]uint16{"State": HCN_ENDPOINT_STATE_ATTACHED_SHARING}
	filter, err := json.Marshal(filterMap)
	if err != nil {
		return err
	}
	h.endpointQuery.Filter = string(filter)

	h.l.Info("Exiting hnsstats Init...")
	return nil
}

func (h *hnsstats) SetupChannel(ch chan *v1.Event) error {
	h.l.Warn("Plugin does not support SetupChannel", zap.String("plugin", string(Name)))
	return nil
}

func pullHnsStats(ctx context.Context, h *hnsstats) error {
	ticker := time.NewTicker(h.cfg.MetricsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			h.l.Error("hnsstats plugin canceling", zap.Error(ctx.Err()))
			return h.Stop()
		case <-ticker.C:
			// Pull data from node
			// Get local endpoints that are healthy
			endpoints, err := hcn.ListEndpointsQuery(h.endpointQuery)
			if err != nil {
				h.l.Error("Getting endpoints failed", zap.Error(err))
			}
			// Get VFP Ports
			kv, err := getMacToPortGuidMap()
			if err != nil {
				h.l.Error("Getting Vswitch ports failed", zap.Error(err))
			}

			for _, ep := range endpoints {
				if len(ep.IpConfigurations) < 1 {
					h.l.Info("Skipping endpoint without IPAddress", zap.String("EndpointID", ep.Id))
					continue
				}

				id := ep.Id
				mac := ep.MacAddress
				ip := ep.IpConfigurations[0].IpAddress

				if stats, err := hcsshim.GetHNSEndpointStats(id); err != nil {
					h.l.Error("Getting endpoint stats failed for endpoint ID "+id, zap.Error(err))
				} else {
					hnsStatsData := &HnsStatsData{hnscounters: stats, IPAddress: ip}
					h.l.Info(fmt.Sprintf("Fetched HNS endpoints stats for ID: %s, IP %s, MAC %s", id, ip, mac))
					// h.l.Info(hnsStatsData.String())

					// Get VFP port counters for matching port (MAC address of endpoint as the key)
					portguid := kv[mac]
					if countersRaw, err := getVfpPortCountersRaw(portguid); len(portguid) > 0 && err == nil {
						if vfpcounters, err := parseVfpPortCounters(countersRaw); err == nil {
							// Attach VFP port counters
							hnsStatsData.vfpCounters = vfpcounters
							h.l.Info("Attached VFP port counters", zap.String("Port", portguid))
							// h.l.Info(vfpcounters.String())
						} else {
							h.l.Error("Unable to parse VFP port counters for Port "+portguid, zap.Error(err))
						}
					} else {
						h.l.Error("Unable to find VFP port counters for MAC "+mac, zap.Error(err))
					}

					notifyHnsStats(h, hnsStatsData)
				}
			}
		}
	}
}

func notifyHnsStats(h *hnsstats, stats *HnsStatsData) {
	// hns signals
	metrics.ForwardCounter.WithLabelValues(ingressLabel).Set(float64(stats.hnscounters.PacketsReceived))
	h.l.Debug(fmt.Sprintf("emitting label %s for value %v", PacketsReceived, stats.hnscounters.PacketsReceived))

	metrics.ForwardCounter.WithLabelValues(egressLabel).Set(float64(stats.hnscounters.PacketsSent))
	h.l.Debug(fmt.Sprintf("emitting label %s for value %v", PacketsSent, stats.hnscounters.PacketsSent))

	metrics.ForwardBytesCounter.WithLabelValues(egressLabel).Set(float64(stats.hnscounters.BytesSent))
	h.l.Debug(fmt.Sprintf("emitting label %s for value %v", BytesSent, stats.hnscounters.BytesSent))

	metrics.ForwardBytesCounter.WithLabelValues(ingressLabel).Set(float64(stats.hnscounters.BytesReceived))
	h.l.Debug(fmt.Sprintf("emitting label %s for value %v", BytesReceived, stats.hnscounters.BytesReceived))

	metrics.WindowsCounter.WithLabelValues(PacketsReceived).Set(float64(stats.hnscounters.PacketsReceived))
	metrics.WindowsCounter.WithLabelValues(PacketsSent).Set(float64(stats.hnscounters.PacketsSent))

	metrics.DropCounter.WithLabelValues(utils.Endpoint, egressLabel).Set(float64(stats.hnscounters.DroppedPacketsOutgoing))
	metrics.DropCounter.WithLabelValues(utils.Endpoint, ingressLabel).Set(float64(stats.hnscounters.DroppedPacketsIncoming))

	if stats.vfpCounters == nil {
		h.l.Warn("will not record some metrics since VFP port counters failed to be set")
		return
	}

	metrics.DropCounter.WithLabelValues(utils.AclRule, ingressLabel).Set(float64(stats.vfpCounters.In.DropCounters.AclDropPacketCount))
	metrics.DropCounter.WithLabelValues(utils.AclRule, egressLabel).Set(float64(stats.vfpCounters.Out.DropCounters.AclDropPacketCount))

	metrics.TCPConnectionStats.WithLabelValues(utils.ResetCount).Set(float64(stats.vfpCounters.In.TcpCounters.ConnectionCounters.ResetCount))
	metrics.TCPConnectionStats.WithLabelValues(utils.ClosedFin).Set(float64(stats.vfpCounters.In.TcpCounters.ConnectionCounters.ClosedFinCount))
	metrics.TCPConnectionStats.WithLabelValues(utils.ResetSyn).Set(float64(stats.vfpCounters.In.TcpCounters.ConnectionCounters.ResetSynCount))
	metrics.TCPConnectionStats.WithLabelValues(utils.TcpHalfOpenTimeouts).Set(float64(stats.vfpCounters.In.TcpCounters.ConnectionCounters.TcpHalfOpenTimeoutsCount))
	metrics.TCPConnectionStats.WithLabelValues(utils.Verified).Set(float64(stats.vfpCounters.In.TcpCounters.ConnectionCounters.VerifiedCount))
	metrics.TCPConnectionStats.WithLabelValues(utils.TimedOutCount).Set(float64(stats.vfpCounters.In.TcpCounters.ConnectionCounters.TimedOutCount))
	metrics.TCPConnectionStats.WithLabelValues(utils.TimeWaitExpiredCount).Set(float64(stats.vfpCounters.In.TcpCounters.ConnectionCounters.TimeWaitExpiredCount))

	// TCP Flag counters
	metrics.TCPFlagCounters.WithLabelValues(ingressLabel, utils.SYN).Set(float64(stats.vfpCounters.In.TcpCounters.PacketCounters.SynPacketCount))
	metrics.TCPFlagCounters.WithLabelValues(ingressLabel, utils.SYNACK).Set(float64(stats.vfpCounters.In.TcpCounters.PacketCounters.SynAckPacketCount))
	metrics.TCPFlagCounters.WithLabelValues(ingressLabel, utils.FIN).Set(float64(stats.vfpCounters.In.TcpCounters.PacketCounters.FinPacketCount))
	metrics.TCPFlagCounters.WithLabelValues(ingressLabel, utils.RST).Set(float64(stats.vfpCounters.In.TcpCounters.PacketCounters.RstPacketCount))

	metrics.TCPFlagCounters.WithLabelValues(egressLabel, utils.SYN).Set(float64(stats.vfpCounters.Out.TcpCounters.PacketCounters.SynPacketCount))
	metrics.TCPFlagCounters.WithLabelValues(egressLabel, utils.SYNACK).Set(float64(stats.vfpCounters.Out.TcpCounters.PacketCounters.SynAckPacketCount))
	metrics.TCPFlagCounters.WithLabelValues(egressLabel, utils.FIN).Set(float64(stats.vfpCounters.Out.TcpCounters.PacketCounters.FinPacketCount))
	metrics.TCPFlagCounters.WithLabelValues(egressLabel, utils.RST).Set(float64(stats.vfpCounters.Out.TcpCounters.PacketCounters.RstPacketCount))
}

func (h *hnsstats) Start(ctx context.Context) error {
	h.l.Info("Start hnsstats plugin...")
	h.state = start
	return pullHnsStats(ctx, h)
}

func (d *hnsstats) Stop() error {
	d.l.Info("Entered hnsstats Stop...")
	if d.state != start {
		d.l.Info("plugin not started")
		return nil
	}
	d.l.Info("Stopped listening for hnsstats event...")
	d.state = stop
	d.l.Info("Exiting hnsstats Stop...")
	return nil
}

// New creates an hnsstats plugin.
func New(cfg *kcfg.Config) api.Plugin {
	// Init logger
	return &hnsstats{
		cfg: cfg,
		l:   log.Logger().Named(string(Name)),
	}
}
