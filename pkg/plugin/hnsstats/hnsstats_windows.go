// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

// Package hnsstats contains the hnsstats plugin. It gathers TCP statistics and counts number of packets/bytes forwarded or dropped in HNS and VFP from Windows nodes.
package hnsstats

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Microsoft/hcsshim"
	"github.com/Microsoft/hcsshim/hcn"
	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/metrics"
	"github.com/microsoft/retina/pkg/plugin/registry"
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

const (
	zapEndpointIDField = "endpointID"
	zapIPField         = "ip"
	zapMACField        = "mac"
	zapPluginField     = "plugin"
	zapPortField       = "port"
)

func init() {
	registry.Plugins[name] = New
}

func New(cfg *kcfg.Config) registry.Plugin {
	return &hnsstats{
		cfg: cfg,
		l:   log.Logger().Named(name),
	}
}

func (h *hnsstats) Name() string {
	return name
}

func (h *hnsstats) Generate(context.Context) error {
	return nil
}

func (h *hnsstats) Compile(context.Context) error {
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

func (h *hnsstats) SetupChannel(chan *v1.Event) error {
	h.l.Warn("Plugin does not support SetupChannel", zap.String(zapPluginField, name))
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
					h.l.Info("Skipping endpoint without IPAddress", zap.String(zapEndpointIDField, ep.Id))
					continue
				}

				id := ep.Id
				mac := ep.MacAddress
				ip := ep.IpConfigurations[0].IpAddress

				if stats, err := hcsshim.GetHNSEndpointStats(id); err != nil {
					h.l.Error("Getting endpoint stats failed", zap.String(zapEndpointIDField, id), zap.Error(err))
				} else {
					hnsStatsData := &HnsStatsData{hnscounters: stats, IPAddress: ip}
					h.l.Debug("Fetched HNS endpoints stats", zap.String(zapEndpointIDField, id),
						zap.String(zapIPField, ip), zap.String(zapMACField, mac))
					// h.l.Info(hnsStatsData.String())

					// Get VFP port counters for matching port (MAC address of endpoint as the key)
					portguid, ok := kv[mac]
					if !ok || len(portguid) == 0 {
						h.l.Error("port is either empty of not found", zap.String(zapMACField, mac))
						continue
					}
					if countersRaw, err := getVfpPortCountersRaw(portguid); len(portguid) > 0 && err == nil {
						if vfpcounters, err := parseVfpPortCounters(countersRaw); err == nil {
							// Attach VFP port counters
							hnsStatsData.vfpCounters = vfpcounters
							h.l.Debug("Attached VFP port counters", zap.String(zapPortField, portguid))
							// h.l.Info(vfpcounters.String())
						} else {
							h.l.Error("Unable to parse VFP port counters", zap.String(zapPortField, portguid), zap.Error(err))
						}
					} else {
						h.l.Error("Unable to find VFP port counters", zap.String(zapMACField, mac), zap.String(zapPortField, portguid), zap.Error(err))
					}

					notifyHnsStats(h, hnsStatsData)
				}
			}
		}
	}
}

func notifyHnsStats(h *hnsstats, stats *HnsStatsData) {
	// hns signals
	metrics.ForwardPacketsGauge.WithLabelValues(ingressLabel).Set(float64(stats.hnscounters.PacketsReceived))
	h.l.Debug("emitting packets received count metric", zap.Uint64(PacketsReceived, stats.hnscounters.PacketsReceived))

	metrics.ForwardPacketsGauge.WithLabelValues(egressLabel).Set(float64(stats.hnscounters.PacketsSent))
	h.l.Debug("emitting packets sent count metric", zap.Uint64(PacketsSent, stats.hnscounters.PacketsSent))

	metrics.ForwardBytesGauge.WithLabelValues(egressLabel).Set(float64(stats.hnscounters.BytesSent))
	h.l.Debug("emitting bytes sent count metric", zap.Uint64(BytesSent, stats.hnscounters.BytesSent))

	metrics.ForwardBytesGauge.WithLabelValues(ingressLabel).Set(float64(stats.hnscounters.BytesReceived))
	h.l.Debug("emitting bytes received count metric", zap.Uint64(BytesReceived, stats.hnscounters.BytesReceived))

	metrics.WindowsGauge.WithLabelValues(PacketsReceived).Set(float64(stats.hnscounters.PacketsReceived))
	metrics.WindowsGauge.WithLabelValues(PacketsSent).Set(float64(stats.hnscounters.PacketsSent))

	metrics.DropPacketsGauge.WithLabelValues(utils.Endpoint, egressLabel).Set(float64(stats.hnscounters.DroppedPacketsOutgoing))
	metrics.DropPacketsGauge.WithLabelValues(utils.Endpoint, ingressLabel).Set(float64(stats.hnscounters.DroppedPacketsIncoming))

	if stats.vfpCounters == nil {
		h.l.Warn("will not record some metrics since VFP port counters failed to be set")
		return
	}

	metrics.DropPacketsGauge.WithLabelValues(utils.AclRule, ingressLabel).Set(float64(stats.vfpCounters.In.DropCounters.AclDropPacketCount))
	metrics.DropPacketsGauge.WithLabelValues(utils.AclRule, egressLabel).Set(float64(stats.vfpCounters.Out.DropCounters.AclDropPacketCount))

	metrics.TCPConnectionStatsGauge.WithLabelValues(utils.ResetCount).Set(float64(stats.vfpCounters.In.TcpCounters.ConnectionCounters.ResetCount))
	metrics.TCPConnectionStatsGauge.WithLabelValues(utils.ClosedFin).Set(float64(stats.vfpCounters.In.TcpCounters.ConnectionCounters.ClosedFinCount))
	metrics.TCPConnectionStatsGauge.WithLabelValues(utils.ResetSyn).Set(float64(stats.vfpCounters.In.TcpCounters.ConnectionCounters.ResetSynCount))
	metrics.TCPConnectionStatsGauge.WithLabelValues(utils.TcpHalfOpenTimeouts).Set(float64(stats.vfpCounters.In.TcpCounters.ConnectionCounters.TcpHalfOpenTimeoutsCount))
	metrics.TCPConnectionStatsGauge.WithLabelValues(utils.Verified).Set(float64(stats.vfpCounters.In.TcpCounters.ConnectionCounters.VerifiedCount))
	metrics.TCPConnectionStatsGauge.WithLabelValues(utils.TimedOutCount).Set(float64(stats.vfpCounters.In.TcpCounters.ConnectionCounters.TimedOutCount))
	metrics.TCPConnectionStatsGauge.WithLabelValues(utils.TimeWaitExpiredCount).Set(float64(stats.vfpCounters.In.TcpCounters.ConnectionCounters.TimeWaitExpiredCount))

	// TCP Flag counters
	metrics.TCPFlagGauge.WithLabelValues(ingressLabel, utils.SYN).Set(float64(stats.vfpCounters.In.TcpCounters.PacketCounters.SynPacketCount))
	metrics.TCPFlagGauge.WithLabelValues(ingressLabel, utils.SYNACK).Set(float64(stats.vfpCounters.In.TcpCounters.PacketCounters.SynAckPacketCount))
	metrics.TCPFlagGauge.WithLabelValues(ingressLabel, utils.FIN).Set(float64(stats.vfpCounters.In.TcpCounters.PacketCounters.FinPacketCount))
	metrics.TCPFlagGauge.WithLabelValues(ingressLabel, utils.RST).Set(float64(stats.vfpCounters.In.TcpCounters.PacketCounters.RstPacketCount))

	metrics.TCPFlagGauge.WithLabelValues(egressLabel, utils.SYN).Set(float64(stats.vfpCounters.Out.TcpCounters.PacketCounters.SynPacketCount))
	metrics.TCPFlagGauge.WithLabelValues(egressLabel, utils.SYNACK).Set(float64(stats.vfpCounters.Out.TcpCounters.PacketCounters.SynAckPacketCount))
	metrics.TCPFlagGauge.WithLabelValues(egressLabel, utils.FIN).Set(float64(stats.vfpCounters.Out.TcpCounters.PacketCounters.FinPacketCount))
	metrics.TCPFlagGauge.WithLabelValues(egressLabel, utils.RST).Set(float64(stats.vfpCounters.Out.TcpCounters.PacketCounters.RstPacketCount))
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
