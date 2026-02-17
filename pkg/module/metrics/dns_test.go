// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package metrics

import (
	"reflect"
	"testing"
	"time"

	"github.com/cilium/cilium/api/v1/flow"
	"github.com/microsoft/retina/crd/api/v1alpha1"
	"github.com/microsoft/retina/pkg/log"
	metricsinit "github.com/microsoft/retina/pkg/metrics"
	"github.com/microsoft/retina/pkg/utils"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/mock/gomock"
	"gotest.tools/v3/assert"
)

const (
	request  = "request"
	response = "response"
)

func TestGetLabels(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	l := log.Logger().Named("testGetLabels")

	tests := []struct {
		name       string
		want       []string
		d          *DNSMetrics
		labelTypes string
	}{
		{
			name: "basic context request labels",
			want: utils.DNSRequestLabels,
			d: &DNSMetrics{
				baseMetricInterface: &baseMetricObject{
					srcCtx: nil,
					dstCtx: nil,
				},
			},
			labelTypes: request,
		},
		{
			name: "basic context response labels",
			want: utils.DNSResponseLabels,
			d: &DNSMetrics{
				baseMetricInterface: &baseMetricObject{
					srcCtx: nil,
					dstCtx: nil,
				},
			},
			labelTypes: response,
		},
		{
			name: "local context request labels",
			want: append(utils.DNSRequestLabels, "ip", "namespace", "podname", "workload_kind", "workload_name", "service", "port"),
			d: &DNSMetrics{
				baseMetricInterface: &baseMetricObject{
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
			labelTypes: request,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			switch tt.labelTypes {
			case request:
				if got := tt.d.getRequestLabels(); !reflect.DeepEqual(got, tt.want) {
					t.Errorf("GetRequestLabels() = %v, want %v", got, tt.want)
				}
			case response:
				if got := tt.d.getResponseLabels(); !reflect.DeepEqual(got, tt.want) {
					t.Errorf("GetResponseLabels() = %v, want %v", got, tt.want)
				}
			default:
				t.Errorf("Invalid label type")
			}
		})
	}
}

func TestValues(t *testing.T) {
	testR := &flow.Flow{}
	extR := utils.NewExtensions()
	utils.AddDNSInfo(testR, extR, "R", 0, "bing.com", []string{"A"}, 1, []string{"1.1.1.1"})
	utils.SetExtensions(testR, extR)

	testQ := &flow.Flow{}
	extQ := utils.NewExtensions()
	utils.AddDNSInfo(testQ, extQ, "Q", 0, "bing.com", []string{"A"}, 0, []string{})
	utils.SetExtensions(testQ, extQ)

	testU := &flow.Flow{}
	extU := utils.NewExtensions()
	utils.AddDNSInfo(testU, extU, "U", 0, "bing.com", []string{"A"}, 0, []string{})
	utils.SetExtensions(testU, extU)

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
			want:   []string{"A", "bing.com"},
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
			switch tt.l7Type {
			case flow.L7FlowType_REQUEST:
				if got := tt.d.requestValues(tt.input); !reflect.DeepEqual(got, tt.want) {
					t.Errorf("RequestValues() = %v, want %v", got, tt.want)
				}
			case flow.L7FlowType_RESPONSE:
				if got := tt.d.responseValues(tt.input); !reflect.DeepEqual(got, tt.want) {
					t.Errorf("ResponseValues() = %v, want %v", got, tt.want)
				}
			case flow.L7FlowType_UNKNOWN_L7_TYPE:
				if got := tt.d.responseValues(tt.input); !reflect.DeepEqual(got, tt.want) {
					t.Errorf("ResponseValues() = %v, want %v", got, tt.want)
				}
			case flow.L7FlowType_SAMPLE:
			default:
				t.Errorf("Invalid L7FlowType")
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

	tests := []struct {
		name           string
		input          *flow.Flow
		expectedLabels []string
		metricsUpdate  bool
	}{
		{
			name: "No context labels",
			input: &flow.Flow{
				Verdict: utils.Verdict_DNS,
			},
			metricsUpdate: false,
		},
		{
			name: "Only ingress labels",
			input: &flow.Flow{
				Verdict:          utils.Verdict_DNS,
				TrafficDirection: flow.TrafficDirection_INGRESS,
				Destination: &flow.Endpoint{
					PodName:   "PodB",
					Namespace: "NamespaceB",
				},
			},
			expectedLabels: []string{"NOERROR", "A", "bing.com", "1.1.1.1", "1", "NamespaceB", "PodB"},
			metricsUpdate:  true,
		},
		{
			name: "Only egress labels",
			input: &flow.Flow{
				Verdict:          utils.Verdict_DNS,
				TrafficDirection: flow.TrafficDirection_EGRESS,
				Source: &flow.Endpoint{
					PodName:   "PodA",
					Namespace: "NamespaceA",
				},
			},
			expectedLabels: []string{"NOERROR", "A", "bing.com", "1.1.1.1", "1", "NamespaceA", "PodA"},
			metricsUpdate:  true,
		},
		{
			name: "Both ingress and egress labels with ingress flow",
			input: &flow.Flow{
				Verdict:          utils.Verdict_DNS,
				TrafficDirection: flow.TrafficDirection_INGRESS,
				Destination: &flow.Endpoint{
					PodName:   "PodA",
					Namespace: "NamespaceA",
				},
				Source: &flow.Endpoint{
					PodName:   "PodB",
					Namespace: "NamespaceB",
				},
			},
			expectedLabels: []string{"NOERROR", "A", "bing.com", "1.1.1.1", "1", "NamespaceA", "PodA"},
			metricsUpdate:  true,
		},
		{
			name: "Both source and destination labels with egress flow",
			input: &flow.Flow{
				Verdict:          utils.Verdict_DNS,
				TrafficDirection: flow.TrafficDirection_EGRESS,
				Source: &flow.Endpoint{
					PodName:   "PodB",
					Namespace: "NamespaceB",
				},
				Destination: &flow.Endpoint{
					PodName:   "PodA",
					Namespace: "NamespaceA",
				},
			},
			expectedLabels: []string{"NOERROR", "A", "bing.com", "1.1.1.1", "1", "NamespaceB", "PodB"},
			metricsUpdate:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ext := utils.NewExtensions()
			utils.AddDNSInfo(tt.input, ext, "R", 0, "bing.com", []string{"A"}, 1, []string{"1.1.1.1"})
			utils.SetExtensions(tt.input, ext)

			mockCV := metricsinit.NewMockCounterVec(ctrl)
			if tt.metricsUpdate {
				mockCV.EXPECT().WithLabelValues(tt.expectedLabels).Return(c).Times(1)
			}

			ctxOptions := &v1alpha1.MetricsContextOptions{
				MetricName: utils.DNSResponseCounterName,
				SourceLabels: []string{
					podCtxOption,
					namespaceCtxOption,
				},
			}
			d := NewDNSMetrics(ctxOptions, l, localContext, 0)
			d.dnsMetrics = mockCV

			d.ProcessFlow(tt.input)

			// There should be no tracked metrics when TTL is infinite
			assert.Equal(t, 0, len(d.trackedMetricLabels()), "there should be no tracked metrics when TTL is infinite")

			// Test TTL based expiration
			metricsinit.InitializeMetrics()

			// Set the TTL to something high to ensure that our call to expire is the only one that expires the metrics
			d = NewDNSMetrics(ctxOptions, l, localContext, time.Minute)
			d.dnsMetrics = mockCV

			if tt.metricsUpdate {
				mockCV.EXPECT().WithLabelValues(tt.expectedLabels).Return(c).Times(1)
			}

			d.ProcessFlow(tt.input)

			if tt.metricsUpdate {
				mockCV.EXPECT().DeleteLabelValues(tt.expectedLabels).Return(true).Times(1)
			}

			for _, ls := range d.trackedMetricLabels() {
				assert.Check(t, d.expire(ls), "metrics should expire successfully")
			}

			// Test that clean calls the base object
			baseMetricObjectMock := NewMockbaseMetricInterface(ctrl)
			d.baseMetricInterface = baseMetricObjectMock

			baseMetricObjectMock.EXPECT().clean().Times(1)

			d.Clean()
		})
	}
}
