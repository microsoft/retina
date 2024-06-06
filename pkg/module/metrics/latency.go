// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package metrics

import (
	"context"
	"math"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/cilium/cilium/api/v1/flow"
	ttlcache "github.com/jellydator/ttlcache/v3"
	api "github.com/microsoft/retina/crd/api/v1alpha1"
	"github.com/microsoft/retina/pkg/common"
	cc "github.com/microsoft/retina/pkg/controllers/cache"
	"github.com/microsoft/retina/pkg/exporter"
	"github.com/microsoft/retina/pkg/log"
	metricsinit "github.com/microsoft/retina/pkg/metrics"
	"github.com/microsoft/retina/pkg/pubsub"
	"github.com/microsoft/retina/pkg/utils"
	"go.uber.org/zap"
)

const (
	apiserverLatencyName                          = "adv_node_apiserver_latency"
	apiserverLatencyDesc                          = "Latency of node apiserver in ms"
	noResponseFromNodeAPIServerName               = "adv_node_apiserver_no_response"
	noResponseFromNodeAPIServerDesc               = "Number of packets that did not get a response from node apiserver"
	apiServerHandshakeLatencyName                 = "adv_node_apiserver_tcp_handshake_latency"
	apiServerHandshakeLatencyDesc                 = "Latency of node apiserver tcp handshake in ms"
	TTL                             time.Duration = 500 * time.Millisecond
	LIMIT                           uint64        = 100000
	// Bucket size.
	start = 0
	width = 0.5
	count = 10
)

type key struct {
	// Flow + TCP ID uniquely identifies a TCP connection.
	// Source IP, Source Port, Destination IP, Destination Port and TCP ID.
	// TCP ID is the 32 bit number that uniquely identifies a TCP connection.
	// We are using TSval/TSecr to identify a TCP connection.
	// Ref for direction: cprog/packetparser.h .
	srcIP string
	dstIP string
	srcP  uint32
	dstP  uint32
	id    uint64
}

type val struct {
	// Time in nanoseconds.
	t int32
	// TCP flags. Required for handshake latency.
	flags *flow.TCPFlags
}

type LatencyMetrics struct {
	l                             *log.ZapLogger
	apiServerIps                  map[string]struct{}
	cache                         *ttlcache.Cache[key, *val]
	nodeApiServerLatency          metricsinit.IHistogramVec
	nodeApiServerHandshakeLatency metricsinit.IHistogramVec
	noResponseMetric              metricsinit.ICounterVec
	callbackId                    string
	mu                            sync.RWMutex
}

var lm *LatencyMetrics

func NewLatencyMetrics(ctxOptions *api.MetricsContextOptions, fl *log.ZapLogger, isLocalContext enrichmentContext) *LatencyMetrics {
	if ctxOptions == nil || !strings.Contains(strings.ToLower(ctxOptions.MetricName), nodeApiserver) {
		return nil
	}

	if lm == nil {
		lm = &LatencyMetrics{l: fl.Named("latency-metricsmodule")}
		// Ignore isLocalContext for now.
	}

	switch ctxOptions.MetricName {
	case utils.NodeApiServerLatencyName:
		lm.nodeApiServerLatency = exporter.CreatePrometheusHistogramWithLinearBucketsForMetric(
			exporter.AdvancedRegistry,
			apiserverLatencyName,
			apiserverLatencyDesc,
			start,
			width,
			count,
		)
	case utils.NodeApiServerTcpHandshakeLatencyName:
		lm.nodeApiServerHandshakeLatency = exporter.CreatePrometheusHistogramWithLinearBucketsForMetric(
			exporter.AdvancedRegistry,
			apiServerHandshakeLatencyName,
			apiServerHandshakeLatencyDesc,
			start,
			width,
			count,
		)
	case utils.NoResponseFromApiServerName:
		lm.noResponseMetric = exporter.CreatePrometheusCounterVecForMetric(
			exporter.AdvancedRegistry,
			noResponseFromNodeAPIServerName,
			noResponseFromNodeAPIServerDesc,
			"count",
		)
	}

	return lm
}

func (lm *LatencyMetrics) Init(metricName string) {
	// Create cache.
	// Cache is key value store for key -> timestamp.
	lm.cache = ttlcache.New(
		ttlcache.WithTTL[key, *val](TTL),
		ttlcache.WithCapacity[key, *val](LIMIT),
	)
	lm.cache.OnEviction(func(ctx context.Context, reason ttlcache.EvictionReason, item *ttlcache.Item[key, *val]) {
		if reason == ttlcache.EvictionReasonExpired {
			// Didn't get the corresponding packet.
			lm.l.Debug("Evicted item", zap.Any("item", item))
			if lm.noResponseMetric != nil {
				lm.noResponseMetric.WithLabelValues("no_response").Inc()
				lm.l.Debug("Incremented no response metric", zap.Any("metric", lm.noResponseMetric))
			}
		}
	})

	lm.mu.Lock()
	// initialize the list of apiserver ips
	lm.apiServerIps = make(map[string]struct{})
	lm.mu.Unlock()

	ps := pubsub.New()

	// Register callback.
	// Everytime the apiserver object is updated, the callback function is called and it updates the ip variable.
	fn := pubsub.CallBackFunc(lm.apiserverWatcherCallbackFn)
	// Check if callback is already registered.
	if lm.callbackId == "" {
		lm.callbackId = ps.Subscribe(common.PubSubAPIServer, &fn)
	}

	go lm.cache.Start()
}

func (lm *LatencyMetrics) Clean() {
	if lm.nodeApiServerLatency != nil {
		exporter.UnregisterMetric(exporter.AdvancedRegistry, metricsinit.ToPrometheusType(lm.nodeApiServerLatency))
	}
	if lm.nodeApiServerLatency != nil {
		exporter.UnregisterMetric(exporter.AdvancedRegistry, metricsinit.ToPrometheusType(lm.nodeApiServerHandshakeLatency))
	}
	if lm.noResponseMetric != nil {
		exporter.UnregisterMetric(exporter.AdvancedRegistry, metricsinit.ToPrometheusType(lm.noResponseMetric))
	}
	if lm.cache != nil {
		lm.cache.Stop()
	}

	// Get pubsub instance.
	ps := pubsub.New()

	// Unsubscribe callback.
	if lm.callbackId != "" {
		if err := ps.Unsubscribe(common.PubSubAPIServer, lm.callbackId); err != nil {
			lm.l.Error("failed to unsubscribe callback", zap.Error(err))
			return
		}
		// Reset callback id after unsubscribing.
		lm.callbackId = ""
	}
}

func (lm *LatencyMetrics) ProcessFlow(f *flow.Flow) {
	if f == nil || f.GetL4() == nil || f.GetL4().GetTCP() == nil || utils.GetTCPID(f) == 0 || f.GetIP() == nil {
		return
	}

	// Safely read lm.apiServerIps value
	lm.mu.RLock()
	apiServerIps := lm.apiServerIps
	lm.mu.RUnlock()

	// Convert f.IP.Source and f.IP.Destination to net.IP type
	sourceIP := net.ParseIP(f.IP.Source).String()
	destinationIP := net.ParseIP(f.IP.Destination).String()

	if apiServerIps != nil {
		// Check if source or destination IP is in the apiServerIps set
		_, ipInSource := apiServerIps[sourceIP]
		_, ipInDestination := apiServerIps[destinationIP]

		if ipInSource || ipInDestination {
			lm.calculateLatency(f)
		}
	}
}

/*
+-------------------------------------------------+
|                                                 |
|           TCP Packet Leaving Host               |
|                                                 |
+-------------------------------------------------+

	|
	|
	v

+-------------------------------------------------+
|                                                 |
|          Add Entry to TTL Cache                 |
|                                                 |
|   Key: {source IP, destination IP,              |
|         source port, destination port,           |
|         tcp timestamp value}                     |
|                                                 |
|   Value: Time in Nanoseconds                     |
|                                                 |
+-------------------------------------------------+

	|
	|
	v

+-------------------------------------------------+
|                                                 |
|           TCP Packet Entering Host               |
|                                                 |
+-------------------------------------------------+

	|
	|
	v

+-------------------------------------------------+
|                                                 |
|       Check if Packet is in TTL Cache           |
|                                                 |
|   Key: {destination IP, source IP,               |
|         destination port, source port,           |
|         tcp timestamp echo reply value}          |
|                                                 |
|   If Yes, Calculate RTT:                         |
|   RTT = Current Time - Time Packet Observed      |
|                                                 |
|   Remove Entry from TTL Cache                    |
|                                                 |
+-------------------------------------------------+
*/
func (lm *LatencyMetrics) calculateLatency(f *flow.Flow) {
	// Ignore all packets observed at endpoint.
	// We only care about node-apiserver packets observed at eth0.
	// TO_NETWORK: Packets leaving node via eth0.
	// FROM_NETWORK: Packets entering node via eth0.
	if f.GetTraceObservationPoint() == flow.TraceObservationPoint_TO_NETWORK {
		k := key{
			srcIP: f.IP.Source,
			dstIP: f.IP.Destination,
			srcP:  f.GetL4().GetTCP().GetSourcePort(),
			dstP:  f.GetL4().GetTCP().GetDestinationPort(),
			id:    utils.GetTCPID(f),
		}
		// There will be multiple identical packets with same ID. Store only the first one.
		if item := lm.cache.Get(k); item == nil {
			lm.cache.Set(k, &val{
				t:     f.Time.Nanos,
				flags: f.GetL4().GetTCP().GetFlags(),
			}, TTL)
		}
	} else if f.GetTraceObservationPoint() == flow.TraceObservationPoint_FROM_NETWORK {
		k := key{
			srcIP: f.IP.Destination,
			dstIP: f.IP.Source,
			srcP:  f.GetL4().GetTCP().GetDestinationPort(),
			dstP:  f.GetL4().GetTCP().GetSourcePort(),
			id:    utils.GetTCPID(f),
		}
		if item := lm.cache.Get(k); item != nil {
			// Calculate latency in milliseconds.
			latency := math.Round(float64(f.Time.Nanos-item.Value().t) / float64(1000000))

			// Log continuous latency.
			if lm.nodeApiServerLatency != nil {
				lm.nodeApiServerLatency.Observe(latency)
			}

			// Determine if this is the first reply packet, and if so, log handshake latency.
			prevFlowflags := item.Value().flags
			curFlowflags := f.L4.GetTCP().Flags
			if lm.nodeApiServerHandshakeLatency != nil && prevFlowflags != nil && prevFlowflags.SYN && curFlowflags != nil && curFlowflags.SYN && curFlowflags.ACK {
				// This is the first reply packet.
				lm.nodeApiServerHandshakeLatency.Observe(latency)
			}
			// Delete the entry from cache. Calculate latency for the first reply packet only.
			lm.cache.Delete(k)
		}
	}
}

func (lm *LatencyMetrics) apiserverWatcherCallbackFn(obj interface{}) {
	event := obj.(*cc.CacheEvent)
	if event == nil {
		return
	}

	apiServer := event.Obj.(*common.APIServerObject)
	if apiServer == nil {
		lm.l.Warn("invalid or nil APIServer object in callback function")
		return
	}

	// Locking before modifying the ip variable
	lm.mu.Lock()
	defer lm.mu.Unlock()
	apiServerIPs := apiServer.IPs()

	switch event.Type {
	case cc.EventTypeAddAPIServerIPs:
		lm.l.Debug("Add apiserver ips", zap.Any("ips", apiServerIPs))
		lm.addIps(apiServerIPs)
	case cc.EventTypeDeleteAPIServerIPs:
		lm.l.Debug("Delete apiserver ips", zap.Any("ips", apiServerIPs))
		lm.removeIps(apiServerIPs)

	default:
		lm.l.Debug("Unknown event type", zap.Any("event", event))
	}
}

func (lm *LatencyMetrics) addIps(ips []net.IP) {
	for _, ip := range ips {
		lm.apiServerIps[ip.String()] = struct{}{}
	}
}

func (lm *LatencyMetrics) removeIps(ips []net.IP) {
	for _, ip := range ips {
		delete(lm.apiServerIps, ip.String())
	}
}
