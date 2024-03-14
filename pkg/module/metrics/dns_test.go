// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package metrics

import (
	"reflect"
	"testing"

	"github.com/cilium/cilium/api/v1/flow"
	"github.com/golang/mock/gomock"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/metrics"
	"github.com/microsoft/retina/pkg/utils"
	"github.com/prometheus/client_golang/prometheus"
	"gotest.tools/v3/assert"
)

func TestGetLabels(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	l := log.Logger().Named("testGetLabels")

	tests := []struct {
		name string
		want []string
		d    *DNSMetrics
	}{
		{
			name: "basic context",
			want: utils.DNSLabels,
			d: &DNSMetrics{
				baseMetricObject: baseMetricObject{
					srcCtx: nil,
					dstCtx: nil,
				},
			},
		},
		{
			name: "local context",
			want: append(utils.DNSLabels, "ip", "namespace", "podname", "workloadKind", "workloadName", "service", "port"),
			d: &DNSMetrics{
				baseMetricObject: baseMetricObject{
					srcCtx: &ContextOptions{
						option:    localCtx,
						IP:        true,
						Namespace: true,
						Podname:   true,
						Service:   true,
						Port:      true,
						Workload:  true,
					},
					dstCtx: nil,
					l:      l,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.d.getLabels(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetLabels() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValues(t *testing.T) {
	testR := &flow.Flow{}
	utils.AddDnsInfo(testR, "R", 0, "bing.com", []string{"A"}, 1, []string{"1.1.1.1"})

	testQ := &flow.Flow{}
	utils.AddDnsInfo(testQ, "Q", 0, "bing.com", []string{"A"}, 0, []string{})

	testU := &flow.Flow{}
	utils.AddDnsInfo(testU, "U", 0, "bing.com", []string{"A"}, 0, []string{})

	tests := []struct {
		name   string
		want   []string
		d      *DNSMetrics
		input  *flow.Flow
		l7Type flow.L7FlowType
	}{
		{
			name:   "basic context",
			want:   nil,
			d:      &DNSMetrics{},
			input:  nil,
			l7Type: 0,
		},
		{
			name:   "Query",
			want:   []string{"NOERROR", "A", "bing.com", "", "0"},
			d:      &DNSMetrics{metricName: utils.DNSRequestCounterName},
			input:  testQ,
			l7Type: flow.L7FlowType_REQUEST,
		},
		{
			name:   "Response",
			want:   []string{"NOERROR", "A", "bing.com", "1.1.1.1", "1"},
			d:      &DNSMetrics{metricName: utils.DNSResponseCounterName},
			input:  testR,
			l7Type: flow.L7FlowType_RESPONSE,
		},
		{
			name:   "UnknownType/DNSRequest",
			want:   nil,
			d:      &DNSMetrics{metricName: utils.DNSRequestCounterName},
			input:  testU,
			l7Type: flow.L7FlowType_UNKNOWN_L7_TYPE,
		},
		{
			name:   "UnknownType/DNSResponse",
			want:   nil,
			d:      &DNSMetrics{metricName: utils.DNSResponseCounterName},
			input:  testU,
			l7Type: flow.L7FlowType_UNKNOWN_L7_TYPE,
		},
		{
			name:   "Query/ResponseMetric",
			want:   nil,
			d:      &DNSMetrics{metricName: utils.DNSResponseCounterName},
			input:  testQ,
			l7Type: flow.L7FlowType_REQUEST,
		},
		{
			name:   "Response/RequestMetric",
			want:   nil,
			d:      &DNSMetrics{metricName: utils.DNSRequestCounterName},
			input:  testR,
			l7Type: flow.L7FlowType_RESPONSE,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.d.values(tt.input); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Values() = %v, want %v", got, tt.want)
			}
			if tt.input == nil {
				return
			}
			assert.Equal(t, tt.input.Type, flow.FlowType_L7)
			assert.Equal(t, tt.input.GetL7().GetType(), tt.l7Type)
		})
	}
}

func TestProcessLocalCtx(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	l := log.Logger().Named("testValues")

	c := prometheus.NewCounter(prometheus.CounterOpts{})

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	testR := &flow.Flow{}
	utils.AddDnsInfo(testR, "R", 0, "bing.com", []string{"A"}, 1, []string{"1.1.1.1"})

	testIngress := &flow.Flow{TrafficDirection: flow.TrafficDirection_INGRESS}
	utils.AddDnsInfo(testIngress, "R", 0, "bing.com", []string{"A"}, 1, []string{"1.1.1.1"})

	testEgress := &flow.Flow{TrafficDirection: flow.TrafficDirection_EGRESS}
	utils.AddDnsInfo(testEgress, "R", 0, "bing.com", []string{"A"}, 1, []string{"1.1.1.1"})

	tests := []struct {
		name           string
		d              *DNSMetrics
		input          *flow.Flow
		output         map[string][]string
		expectedLabels []string
		metricsUpdate  bool
	}{
		{
			name:          "No context labels",
			input:         nil,
			output:        nil,
			d:             &DNSMetrics{},
			metricsUpdate: false,
		},
		{
			name:  "Only ingress labels",
			input: testR,
			output: map[string][]string{
				ingress: {"PodA", "NamespaceA"},
				egress:  nil,
			},
			d: &DNSMetrics{
				metricName: utils.DNSResponseCounterName,
				baseMetricObject: baseMetricObject{
					l: l,
				},
			},
			expectedLabels: []string{"NOERROR", "A", "bing.com", "1.1.1.1", "1", "PodA", "NamespaceA"},
			metricsUpdate:  true,
		},
		{
			name:  "Only egress labels",
			input: testR,
			output: map[string][]string{
				ingress: nil,
				egress:  {"PodA", "NamespaceA"},
			},
			d: &DNSMetrics{
				metricName: utils.DNSResponseCounterName,
				baseMetricObject: baseMetricObject{
					l: l,
				},
			},
			expectedLabels: []string{"NOERROR", "A", "bing.com", "1.1.1.1", "1", "PodA", "NamespaceA"},
			metricsUpdate:  true,
		},
		{
			name:  "Both ingress and egress labels with ingress flow",
			input: testIngress,
			output: map[string][]string{
				ingress: {"PodA", "NamespaceA"},
				egress:  {"PodB", "NamespaceB"},
			},
			d: &DNSMetrics{
				metricName: utils.DNSResponseCounterName,
				baseMetricObject: baseMetricObject{
					l: l,
				},
			},
			expectedLabels: []string{"NOERROR", "A", "bing.com", "1.1.1.1", "1", "PodA", "NamespaceA"},
			metricsUpdate:  true,
		},
		{
			name:  "Both ingress and egress labels with egress flow",
			input: testEgress,
			output: map[string][]string{
				ingress: {"PodA", "NamespaceA"},
				egress:  {"PodB", "NamespaceB"},
			},
			d: &DNSMetrics{
				metricName: utils.DNSResponseCounterName,
				baseMetricObject: baseMetricObject{
					l: l,
				},
			},
			expectedLabels: []string{"NOERROR", "A", "bing.com", "1.1.1.1", "1", "PodB", "NamespaceB"},
			metricsUpdate:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMockContextOptionsInterface(ctrl) //nolint:typecheck
			m.EXPECT().getLocalCtxValues(tt.input).Return(tt.output).Times(1)

			mockCV := metrics.NewMockICounterVec(ctrl)
			if tt.metricsUpdate {
				mockCV.EXPECT().WithLabelValues(tt.expectedLabels).Return(c).Times(1)
			}

			tt.d.dnsMetrics = mockCV
			tt.d.srcCtx = m
			tt.d.processLocalCtxFlow(tt.input)
		})
	}
}
