// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package metrics

import (
	"slices"
	"testing"
	"testing/synctest"
	"time"

	"github.com/cilium/cilium/api/v1/flow"
	api "github.com/microsoft/retina/crd/api/v1alpha1"
	"github.com/microsoft/retina/pkg/exporter"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/utils"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

const (
	podMetricTestNamespace   = "ns"
	podMetricTestPodName     = "pod"
	podMetricTestSurvivor    = "live"
	podMetricTestDNSIP       = "10.0.0.1"
	podMetricTestForwardName = "forward_count"
	podMetricTestRegistryKey = "metric"
)

func TestPodMetricSelectors(t *testing.T) {
	l, err := log.SetupZapLogger(log.GetDefaultLogOpts())
	if err != nil {
		t.Fatalf("failed to set up logger: %v", err)
	}
	tests := []struct {
		name         string
		sourceLabels []string
		namespace    string
		podName      string
	}{
		{name: "missing namespace label", sourceLabels: []string{podCtxOption}, namespace: podMetricTestNamespace, podName: podMetricTestPodName},
		{name: "missing pod label", sourceLabels: []string{namespaceCtxOption}, namespace: podMetricTestNamespace, podName: podMetricTestPodName},
		{name: "IP only", sourceLabels: []string{ipCtxOption}, namespace: podMetricTestNamespace, podName: podMetricTestPodName},
		{name: "empty event namespace", sourceLabels: []string{namespaceCtxOption, podCtxOption}, podName: podMetricTestPodName},
		{name: "empty event pod name", sourceLabels: []string{namespaceCtxOption, podCtxOption}, namespace: podMetricTestNamespace},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := newBaseMetricsObject(&api.MetricsContextOptions{
				MetricName:   podMetricTestForwardName,
				SourceLabels: tt.sourceLabels,
			}, l, localContext, func([]string) bool { return true }, 0)
			t.Cleanup(b.clean)
			if got := b.podMetricSelectors(tt.namespace, tt.podName); len(got) != 0 {
				t.Fatalf("podMetricSelectors() = %#v, want no selectors", got)
			}
		})
	}
}

func podMetricTestFlow(verdict flow.Verdict, sourceNamespace, sourcePod, destinationNamespace, destinationPod string) *flow.Flow {
	f := &flow.Flow{
		Verdict:          verdict,
		TrafficDirection: flow.TrafficDirection_EGRESS,
	}
	if sourceNamespace != "" || sourcePod != "" {
		f.Source = &flow.Endpoint{Namespace: sourceNamespace, PodName: sourcePod}
	}
	if destinationNamespace != "" || destinationPod != "" {
		f.Destination = &flow.Endpoint{Namespace: destinationNamespace, PodName: destinationPod}
	}
	return f
}

func useIsolatedAdvancedRegistry(t *testing.T) {
	t.Helper()

	previous := exporter.AdvancedRegistry
	exporter.AdvancedRegistry = prometheus.NewRegistry()
	t.Cleanup(func() {
		exporter.AdvancedRegistry = previous
	})
}

func gatheredMetricValue(t *testing.T, gatherer prometheus.Gatherer, labels prometheus.Labels) (float64, bool) {
	t.Helper()

	families, err := gatherer.Gather()
	if err != nil {
		t.Fatalf("Gather() failed: %v", err)
	}
	for _, family := range families {
		for _, metric := range family.GetMetric() {
			if !metricHasLabels(metric, labels) {
				continue
			}
			if metric.GetCounter() != nil {
				return metric.GetCounter().GetValue(), true
			}
			if metric.GetGauge() != nil {
				return metric.GetGauge().GetValue(), true
			}
			t.Fatalf("unsupported metric type %s", family.GetType())
		}
	}
	return 0, false
}

func metricHasLabels(metric *dto.Metric, labels prometheus.Labels) bool {
	for name, value := range labels {
		found := false
		for _, pair := range metric.GetLabel() {
			if pair.GetName() == name && pair.GetValue() == value {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func TestBaseDeletePodMetrics(t *testing.T) {
	l, err := log.SetupZapLogger(log.GetDefaultLogOpts())
	if err != nil {
		t.Fatalf("failed to set up logger: %v", err)
	}

	for _, test := range []struct {
		name        string
		ttl         time.Duration
		wantDeleted int
	}{
		{name: "local zero TTL", wantDeleted: 1},
		{name: "local negative TTL", ttl: -time.Second},
	} {
		t.Run(test.name, func(t *testing.T) {
			base := newBaseMetricsObject(&api.MetricsContextOptions{
				MetricName:   podMetricTestForwardName,
				SourceLabels: []string{namespaceCtxOption, podCtxOption},
			}, l, localContext, func([]string) bool { return true }, test.ttl)
			vector := prometheus.NewGaugeVec(prometheus.GaugeOpts{
				Name: "test_base_delete_local",
				Help: "Test local pod metric cleanup",
			}, []string{namespaceCtxOption, podCtxOption})
			base.setMetricVector(vector)
			vector.WithLabelValues(podMetricTestNamespace, podMetricTestPodName).Set(2)
			vector.WithLabelValues(podMetricTestNamespace, podMetricTestSurvivor).Set(5)
			registry := prometheus.NewRegistry()
			registry.MustRegister(vector)

			if got := base.DeletePodMetrics(podMetricTestNamespace, podMetricTestPodName); got != test.wantDeleted {
				t.Fatalf("DeletePodMetrics() = %d, want %d", got, test.wantDeleted)
			}
			if got, ok := gatheredMetricValue(t, registry, prometheus.Labels{
				namespaceCtxOption: podMetricTestNamespace,
				podCtxOption:       podMetricTestPodName,
			}); ok != (test.wantDeleted == 0) || (ok && got != 2) {
				t.Fatalf("victim metric = %v, %t; want preserved: %t", got, ok, test.wantDeleted == 0)
			}
			if got, ok := gatheredMetricValue(t, registry, prometheus.Labels{
				namespaceCtxOption: podMetricTestNamespace,
				podCtxOption:       podMetricTestSurvivor,
			}); !ok || got != 5 {
				t.Fatalf("survivor metric = %v, %t; want 5, true", got, ok)
			}
		})
	}

	t.Run("remote source or destination", func(t *testing.T) {
		base := newBaseMetricsObject(&api.MetricsContextOptions{
			MetricName:        podMetricTestForwardName,
			SourceLabels:      []string{namespaceCtxOption, podCtxOption},
			DestinationLabels: []string{namespaceCtxOption, podCtxOption},
		}, l, remoteContext, func([]string) bool { return true }, 0)
		vector := prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "test_base_delete_remote",
			Help: "Test remote pod metric cleanup",
		}, []string{
			sourceCtxPrefix + namespaceCtxOption,
			sourceCtxPrefix + podCtxOption,
			destinationCtxPrefix + namespaceCtxOption,
			destinationCtxPrefix + podCtxOption,
		})
		base.setMetricVector(vector)
		vector.WithLabelValues(podMetricTestNamespace, podMetricTestPodName, "other", "one").Set(1)
		vector.WithLabelValues("other", "two", podMetricTestNamespace, podMetricTestPodName).Set(2)
		vector.WithLabelValues(podMetricTestNamespace, podMetricTestPodName, podMetricTestNamespace, podMetricTestPodName).Set(3)
		vector.WithLabelValues("other", podMetricTestSurvivor, "other", "survivor").Set(9)
		registry := prometheus.NewRegistry()
		registry.MustRegister(vector)

		if got := base.DeletePodMetrics(podMetricTestNamespace, podMetricTestPodName); got != 3 {
			t.Fatalf("DeletePodMetrics() = %d, want 3", got)
		}
		for _, labels := range []prometheus.Labels{
			{sourceCtxPrefix + namespaceCtxOption: podMetricTestNamespace, sourceCtxPrefix + podCtxOption: podMetricTestPodName, destinationCtxPrefix + podCtxOption: "one"},
			{sourceCtxPrefix + podCtxOption: "two", destinationCtxPrefix + namespaceCtxOption: podMetricTestNamespace, destinationCtxPrefix + podCtxOption: podMetricTestPodName},
			{
				sourceCtxPrefix + namespaceCtxOption:      podMetricTestNamespace,
				sourceCtxPrefix + podCtxOption:            podMetricTestPodName,
				destinationCtxPrefix + namespaceCtxOption: podMetricTestNamespace,
				destinationCtxPrefix + podCtxOption:       podMetricTestPodName,
			},
		} {
			if _, ok := gatheredMetricValue(t, registry, labels); ok {
				t.Fatalf("victim metric is still exported: %v", labels)
			}
		}
		if got, ok := gatheredMetricValue(t, registry, prometheus.Labels{
			sourceCtxPrefix + podCtxOption:      podMetricTestSurvivor,
			destinationCtxPrefix + podCtxOption: "survivor",
		}); !ok || got != 9 {
			t.Fatalf("survivor metric = %v, %t; want 9, true", got, ok)
		}
	})

	for _, test := range []struct {
		name         string
		contextLabel string
		vectorLabels []string
		values       []string
		match        prometheus.Labels
	}{
		{
			name:         "namespace aggregate",
			contextLabel: namespaceCtxOption,
			vectorLabels: []string{sourceCtxPrefix + namespaceCtxOption},
			values:       []string{"ns"},
			match:        prometheus.Labels{sourceCtxPrefix + namespaceCtxOption: "ns"},
		},
		{
			name:         "workload aggregate",
			contextLabel: workloadCtxOption,
			vectorLabels: []string{sourceCtxPrefix + workloadKindCtxOption, sourceCtxPrefix + workloadNameCtxOption},
			values:       []string{"Deployment", "shared"},
			match: prometheus.Labels{
				sourceCtxPrefix + workloadKindCtxOption: "Deployment",
				sourceCtxPrefix + workloadNameCtxOption: "shared",
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			base := newBaseMetricsObject(&api.MetricsContextOptions{
				MetricName:   podMetricTestForwardName,
				SourceLabels: []string{test.contextLabel},
			}, l, remoteContext, func([]string) bool { return true }, 0)
			vector := prometheus.NewGaugeVec(prometheus.GaugeOpts{
				Name: "test_base_delete_" + test.contextLabel,
				Help: "Test aggregate pod metric retention",
			}, test.vectorLabels)
			base.setMetricVector(vector)
			vector.WithLabelValues(test.values...).Set(6)
			registry := prometheus.NewRegistry()
			registry.MustRegister(vector)

			if got := base.DeletePodMetrics(podMetricTestNamespace, podMetricTestPodName); got != 0 {
				t.Fatalf("DeletePodMetrics() = %d, want 0", got)
			}
			if got, ok := gatheredMetricValue(t, registry, test.match); !ok || got != 6 {
				t.Fatalf("aggregate metric = %v, %t; want 6, true", got, ok)
			}
		})
	}
}

func TestPodMetricCollectorsDeleteExportedSeries(t *testing.T) {
	l, err := log.SetupZapLogger(log.GetDefaultLogOpts())
	if err != nil {
		t.Fatalf("failed to set up logger: %v", err)
	}

	forward := func(options *api.MetricsContextOptions) AdvMetricsInterface {
		return NewForwardCountMetrics(options, l, remoteContext, 0)
	}
	drop := func(options *api.MetricsContextOptions) AdvMetricsInterface {
		return NewDropCountMetrics(options, l, remoteContext, 0)
	}
	tcpFlags := func(options *api.MetricsContextOptions) AdvMetricsInterface {
		return NewTCPMetrics(options, l, remoteContext, 0)
	}
	tcpRetransmit := func(options *api.MetricsContextOptions) AdvMetricsInterface {
		return NewTCPRetransMetrics(options, l, remoteContext, 0)
	}
	dns := func(options *api.MetricsContextOptions) AdvMetricsInterface {
		return NewDNSMetrics(options, l, remoteContext, 0)
	}

	tests := []struct {
		name       string
		metricName string
		newMetric  func(*api.MetricsContextOptions) AdvMetricsInterface
		verdict    flow.Verdict
		packetSize uint32
		dnsType    string
		tcpFlags   bool
		wantValue  float64
	}{
		{
			name:       "forward count",
			metricName: utils.ForwardPacketsGaugeName,
			newMetric:  forward,
			verdict:    flow.Verdict_FORWARDED,
			packetSize: 7,
			wantValue:  1,
		},
		{
			name:       "forward bytes",
			metricName: utils.ForwardBytesGaugeName,
			newMetric:  forward,
			verdict:    flow.Verdict_FORWARDED,
			packetSize: 7,
			wantValue:  7,
		},
		{
			name:       "drop count",
			metricName: utils.DroppedPacketsGaugeName,
			newMetric:  drop,
			verdict:    flow.Verdict_DROPPED,
			packetSize: 11,
			wantValue:  1,
		},
		{
			name:       "drop bytes",
			metricName: utils.DropBytesGaugeName,
			newMetric:  drop,
			verdict:    flow.Verdict_DROPPED,
			packetSize: 11,
			wantValue:  11,
		},
		{
			name:       "TCP flags",
			metricName: utils.TCPFlagGauge,
			newMetric:  tcpFlags,
			verdict:    flow.Verdict_FORWARDED,
			tcpFlags:   true,
			wantValue:  1,
		},
		{
			name:       "TCP retransmit",
			metricName: utils.TCPRetransCount,
			newMetric:  tcpRetransmit,
			verdict:    utils.Verdict_RETRANSMISSION,
			wantValue:  1,
		},
		{
			name:       "DNS request",
			metricName: utils.DNSRequestCounterName,
			newMetric:  dns,
			verdict:    utils.Verdict_DNS,
			dnsType:    "Q",
			wantValue:  1,
		},
		{
			name:       "DNS response",
			metricName: utils.DNSResponseCounterName,
			newMetric:  dns,
			verdict:    utils.Verdict_DNS,
			dnsType:    "R",
			wantValue:  1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			useIsolatedAdvancedRegistry(t)
			metric := test.newMetric(&api.MetricsContextOptions{
				MetricName:   test.metricName,
				SourceLabels: []string{namespaceCtxOption, podCtxOption},
			})
			metric.Init(test.metricName)
			for _, podName := range []string{podMetricTestPodName, podMetricTestSurvivor} {
				f := podMetricTestFlow(test.verdict, podMetricTestNamespace, podName, "", "")
				if test.packetSize != 0 || test.dnsType != "" {
					extensions := utils.NewExtensions()
					if test.packetSize != 0 {
						utils.AddPacketSize(extensions, test.packetSize)
					}
					if test.dnsType != "" {
						utils.AddDNSInfo(f, extensions, test.dnsType, 0, "example.test", []string{"A"}, 1, []string{podMetricTestDNSIP})
					}
					utils.SetExtensions(f, extensions)
				}
				if test.tcpFlags {
					f.L4 = &flow.Layer4{Protocol: &flow.Layer4_TCP{TCP: &flow.TCP{Flags: &flow.TCPFlags{SYN: true}}}}
				}
				metric.ProcessFlow(f)
			}

			survivorLabels := prometheus.Labels{
				sourceCtxPrefix + namespaceCtxOption: podMetricTestNamespace,
				sourceCtxPrefix + podCtxOption:       podMetricTestSurvivor,
			}
			before, ok := gatheredMetricValue(t, exporter.AdvancedRegistry, survivorLabels)
			if !ok || before != test.wantValue {
				t.Fatalf("survivor metric before deletion = %v, %t; want %v, true", before, ok, test.wantValue)
			}

			if got := metric.DeletePodMetrics(podMetricTestNamespace, podMetricTestPodName); got != 1 {
				t.Fatalf("DeletePodMetrics() = %d, want 1", got)
			}
			if _, victimFound := gatheredMetricValue(t, exporter.AdvancedRegistry, prometheus.Labels{
				sourceCtxPrefix + namespaceCtxOption: podMetricTestNamespace,
				sourceCtxPrefix + podCtxOption:       podMetricTestPodName,
			}); victimFound {
				t.Fatal("victim metric is still exported")
			}
			after, ok := gatheredMetricValue(t, exporter.AdvancedRegistry, survivorLabels)
			if !ok || after != before {
				t.Fatalf("survivor metric after deletion = %v, %t; want %v, true", after, ok, before)
			}
		})
	}
}

func TestBaseMetricObject(t *testing.T) {
	l, err := log.SetupZapLogger(log.GetDefaultLogOpts())
	if err != nil {
		t.Fatalf("failed to set up logger: %v", err)
	}

	tests := []struct {
		name         string
		ttl          time.Duration
		trackMetrics bool
	}{
		{
			name:         "test base metric object zero ttl",
			ttl:          0,
			trackMetrics: false,
		},
		{
			name:         "test base metric object negative ttl",
			ttl:          -time.Millisecond,
			trackMetrics: false,
		},
		{
			name:         "test base metric object positive ttl",
			ttl:          time.Millisecond,
			trackMetrics: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The bubble gives the test a fake clock, so the expiration
			// ticker fires deterministically. It also fails the test if the
			// expiration goroutine is still alive when the bubble ends.
			synctest.Test(t, func(t *testing.T) {
				var expireCalled []string
				b := newBaseMetricsObject(
					&api.MetricsContextOptions{
						MetricName: "test_metric",
					},
					l,
					localContext,
					func(lbs []string) bool {
						expireCalled = lbs
						return true
					},
					tt.ttl,
				)
				t.Cleanup(b.clean)

				testLabels := []string{"test"}
				b.updated(testLabels)

				metrics := len(b.trackedMetricLabels())
				if tt.trackMetrics {
					if metrics != 1 {
						t.Errorf("expected 1 tracked metric label, got %d", metrics)
					}
				} else {
					if metrics != 0 {
						t.Errorf("expected 0 tracked metric labels, got %d", metrics)
					}
				}

				// If we have a positive TTL, we should see the expire function get called after the TTL has passed
				if tt.ttl > 0 {
					// Advance the fake clock to the first tick, then wait for
					// the expiration goroutine to block again.
					time.Sleep(tt.ttl)
					synctest.Wait()
					if !slices.Equal(expireCalled, testLabels) {
						t.Errorf("expected expire to be called with %v, got %v", testLabels, expireCalled)
					}
					metrics = len(b.trackedMetricLabels())
					if metrics != 0 {
						t.Errorf("expected 0 tracked metric labels after expiration, got %d", metrics)
					}
				} else if len(expireCalled) != 0 {
					t.Errorf("expected expire to not be called, but got %v", expireCalled)
				}
			})
		})
	}
}
