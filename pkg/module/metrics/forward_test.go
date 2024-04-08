// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
//
//nolint:all
package metrics

import (
	"testing"

	"github.com/cilium/cilium/api/v1/flow"
	"github.com/microsoft/retina/crd/api/v1alpha1"
	"github.com/microsoft/retina/pkg/log"
	metricsinit "github.com/microsoft/retina/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

type TestMetrics struct {
	name            string
	opts            *v1alpha1.MetricsContextOptions
	f               *flow.Flow
	checkIsAdvance  bool
	exepectedLabels []string
	metricCall      int
	nilObj          bool
	localContext    enrichmentContext
}

func TestNewForward(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	l := log.Logger().Named("TestNewForward")

	tt := []TestMetrics{
		{
			name:            "empty opts",
			opts:            &v1alpha1.MetricsContextOptions{},
			checkIsAdvance:  false,
			f:               &flow.Flow{},
			exepectedLabels: []string{"direction"},
			metricCall:      0,
			nilObj:          true,
		},
		{
			name: "plain opts",
			opts: &v1alpha1.MetricsContextOptions{
				MetricName: "forward",
			},
			checkIsAdvance: false,
			f: &flow.Flow{
				Verdict: flow.Verdict_FORWARDED,
			},
			exepectedLabels: []string{"direction"},
			metricCall:      1,
		},
		{
			name: "plain opts with nil flow",
			opts: &v1alpha1.MetricsContextOptions{
				MetricName: "forward",
			},
			checkIsAdvance:  false,
			f:               nil,
			exepectedLabels: []string{"direction"},
			metricCall:      0,
		},
		{
			name: "plain opts dropped verdict",
			opts: &v1alpha1.MetricsContextOptions{
				MetricName: "forward",
			},
			checkIsAdvance: false,
			f: &flow.Flow{
				Verdict: flow.Verdict_DROPPED,
			},
			exepectedLabels: []string{"direction"},
			metricCall:      0,
		},
		{
			name: "source opts 1 without metric name",
			opts: &v1alpha1.MetricsContextOptions{
				SourceLabels: []string{"ip", "namespace", "podName", "Workload", "PORT", "serVICE"},
			},
			f: &flow.Flow{},
			exepectedLabels: []string{
				"direction",
			},
			metricCall: 0,
			nilObj:     true,
		},
		{
			name: "source opts 1",
			opts: &v1alpha1.MetricsContextOptions{
				MetricName:   "forward",
				SourceLabels: []string{"ip", "namespace", "podName", "Workload", "PORT", "serVICE"},
			},
			f: &flow.Flow{
				Verdict: flow.Verdict_FORWARDED,
			},
			checkIsAdvance: true,
			exepectedLabels: []string{
				"direction",
				"source_ip",
				"source_namespace",
				"source_podname",
				"source_workloadKind",
				"source_workloadName",
				"source_service",
				"source_port",
			},
			metricCall: 1,
		},
		{
			name: "dest opts 1",
			opts: &v1alpha1.MetricsContextOptions{
				MetricName:        "FORWARD",
				DestinationLabels: []string{"ip", "namespace", "podName", "Workload", "PORT", "serVICE"},
			},
			f: &flow.Flow{
				Verdict: flow.Verdict_FORWARDED,
			},
			checkIsAdvance: true,
			exepectedLabels: []string{
				"direction",
				"destination_ip",
				"destination_namespace",
				"destination_podname",
				"destination_workloadKind",
				"destination_workloadName",
				"destination_service",
				"destination_port",
			},
			metricCall: 1,
		},
		{
			name: "source opts with flow",
			opts: &v1alpha1.MetricsContextOptions{
				MetricName:   "forward",
				SourceLabels: []string{"ip", "namespace", "podName", "Workload", "PORT", "serVICE"},
			},
			f: &flow.Flow{
				Source:  &flow.Endpoint{},
				Verdict: flow.Verdict_FORWARDED,
			},
			checkIsAdvance: true,
			exepectedLabels: []string{
				"direction",
				"source_ip",
				"source_namespace",
				"source_podname",
				"source_workloadKind",
				"source_workloadName",
				"source_service",
				"source_port",
			},
			metricCall: 1,
		},
		{
			name: "drop source opts expect nil",
			opts: &v1alpha1.MetricsContextOptions{
				MetricName:   "drop",
				SourceLabels: []string{"ip", "namespace", "podName", "Workload", "PORT", "serVICE"},
			},
			f: &flow.Flow{
				Source:  &flow.Endpoint{},
				Verdict: flow.Verdict_FORWARDED,
			},
			nilObj:         true,
			checkIsAdvance: true,
			exepectedLabels: []string{
				"direction",
				"source_ip",
				"source_namespace",
				"source_podname",
				"source_workloadKind",
				"source_workloadName",
				"source_service",
				"source_port",
			},
			metricCall: 1,
		},
		{
			name: "source opts with flow dropped verdict",
			opts: &v1alpha1.MetricsContextOptions{
				MetricName:   "forward",
				SourceLabels: []string{"ip", "namespace", "podName", "Workload", "PORT", "serVICE"},
			},
			f: &flow.Flow{
				Source:  &flow.Endpoint{},
				Verdict: flow.Verdict_DROPPED,
			},
			checkIsAdvance: true,
			exepectedLabels: []string{
				"direction",
				"source_ip",
				"source_namespace",
				"source_podname",
				"source_workloadKind",
				"source_workloadName",
				"source_service",
				"source_port",
			},
			metricCall: 0,
		},
		{
			name: "source opts with flow in local context",
			opts: &v1alpha1.MetricsContextOptions{
				MetricName:   "forward",
				SourceLabels: []string{"ip", "namespace", "podName", "Workload", "PORT", "serVICE"},
			},
			f: &flow.Flow{
				Source:  &flow.Endpoint{},
				Verdict: flow.Verdict_FORWARDED,
			},
			checkIsAdvance: true,
			exepectedLabels: []string{
				"direction",
				"ip",
				"namespace",
				"podname",
				"workloadKind",
				"workloadName",
				"service",
				"port",
			},
			metricCall:   1,
			localContext: localContext,
		},
		{
			name: "dest opts 1 with flow in local context",
			opts: &v1alpha1.MetricsContextOptions{
				MetricName:   "FORWARD",
				SourceLabels: []string{"ip", "namespace", "podName", "Workload", "PORT", "serVICE"},
			},
			f: &flow.Flow{
				Verdict:     flow.Verdict_FORWARDED,
				Destination: &flow.Endpoint{},
			},
			checkIsAdvance: true,
			exepectedLabels: []string{
				"direction",
				"ip",
				"namespace",
				"podname",
				"workloadKind",
				"workloadName",
				"service",
				"port",
			},
			metricCall:   1,
			localContext: localContext,
		},
		{
			name: "src and dest opts 1 with flow in local context",
			opts: &v1alpha1.MetricsContextOptions{
				MetricName:   "FORWARD",
				SourceLabels: []string{"ip", "namespace", "podName", "Workload", "PORT", "serVICE"},
			},
			f: &flow.Flow{
				Verdict:     flow.Verdict_FORWARDED,
				Destination: &flow.Endpoint{},
				Source:      &flow.Endpoint{},
			},
			checkIsAdvance: true,
			exepectedLabels: []string{
				"direction",
				"ip",
				"namespace",
				"podname",
				"workloadKind",
				"workloadName",
				"service",
				"port",
			},
			metricCall:   2,
			localContext: localContext,
		},
	}

	for _, tc := range tt {
		for _, metricName := range []string{"forward_count", "forward_bytes"} {
			l.Info("Running test", zap.String("name", tc.name), zap.String("metricName", metricName))
			ctrl := gomock.NewController(t)

			f := NewForwardCountMetrics(tc.opts, log.Logger(), tc.localContext)
			if tc.nilObj {
				assert.Nil(t, f, "forward metrics should be nil Test Name: %s", tc.name)
				continue
			} else {
				assert.NotNil(t, f, "forward metrics should not be nil Test Name: %s", tc.name)
			}

			forwardMock := metricsinit.NewMockICounterVec(ctrl) //nolint:typecheck

			f.forwardMetric = forwardMock

			testmetric := prometheus.NewGauge(prometheus.GaugeOpts{
				Name: "testmetric",
				Help: "testmetric",
			})
			forwardMock.EXPECT().WithLabelValues(gomock.Any()).Return(testmetric).Times(tc.metricCall)
			assert.Equal(t, f.advEnable, tc.checkIsAdvance, "advance metrics options should be equal Test Name: %s", tc.name)
			assert.Equal(t, tc.exepectedLabels, f.getLabels(), "labels should be equal Test Name: %s", tc.name)

			f.metricName = metricName
			f.ProcessFlow(tc.f)
			ctrl.Finish()
		}
	}
}
