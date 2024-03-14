// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
//
//nolint:all
package metrics

import (
	"testing"

	"github.com/cilium/cilium/api/v1/flow"
	"github.com/golang/mock/gomock"
	"github.com/microsoft/retina/crd/api/v1alpha1"
	"github.com/microsoft/retina/pkg/log"
	metricsinit "github.com/microsoft/retina/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestNewDrop(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())

	tt := []TestMetrics{
		{
			name:           "empty opts",
			opts:           &v1alpha1.MetricsContextOptions{},
			checkIsAdvance: false,
			f: &flow.Flow{
				Verdict: flow.Verdict_FORWARDED,
			},
			exepectedLabels: []string{"reason"},
			metricCall:      0,
			nilObj:          true,
		},
		{
			name:           "empty opts",
			opts:           &v1alpha1.MetricsContextOptions{},
			checkIsAdvance: false,
			f: &flow.Flow{
				Verdict: flow.Verdict_DROPPED,
			},
			exepectedLabels: []string{"reason"},
			metricCall:      0,
			nilObj:          true,
		},
		{
			name: "plain opts",
			opts: &v1alpha1.MetricsContextOptions{
				MetricName: "drop",
			},
			checkIsAdvance:  false,
			f:               &flow.Flow{},
			exepectedLabels: []string{"reason", "direction"},
			metricCall:      0,
		},
		{
			name: "plain opts dropped verdict",
			opts: &v1alpha1.MetricsContextOptions{
				MetricName: "drop",
			},
			checkIsAdvance: false,
			f: &flow.Flow{
				Verdict: flow.Verdict_DROPPED,
			},
			exepectedLabels: []string{"reason", "direction"},
			metricCall:      1,
		},
		{
			name: "plain opts dropped verdict nil flow",
			opts: &v1alpha1.MetricsContextOptions{
				MetricName: "drop",
			},
			checkIsAdvance:  false,
			f:               nil,
			exepectedLabels: []string{"reason", "direction"},
			metricCall:      0,
		},
		{
			name: "source opts 1 without metric name",
			opts: &v1alpha1.MetricsContextOptions{
				SourceLabels: []string{"ip", "namespace", "podName", "Workload", "PORT", "serVICE"},
			},
			f: &flow.Flow{
				Verdict: flow.Verdict_DROPPED,
			},
			exepectedLabels: []string{
				"reason",
				"direction",
			},
			metricCall: 1,
			nilObj:     true,
		},
		{
			name: "source opts 1",
			opts: &v1alpha1.MetricsContextOptions{
				MetricName:   "drop",
				SourceLabels: []string{"ip", "namespace", "podName", "Workload", "PORT", "serVICE"},
			},
			f: &flow.Flow{
				Verdict: flow.Verdict_DROPPED,
			},
			checkIsAdvance: true,
			exepectedLabels: []string{
				"reason",
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
				MetricName:        "DROP",
				DestinationLabels: []string{"ip", "namespace", "podName", "Workload", "PORT", "serVICE"},
			},
			f: &flow.Flow{
				Verdict: flow.Verdict_DROPPED,
			},
			checkIsAdvance: true,
			exepectedLabels: []string{
				"reason",
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
				MetricName:   "drop",
				SourceLabels: []string{"ip", "namespace", "podName", "Workload", "PORT", "serVICE"},
			},
			f: &flow.Flow{
				Source:  &flow.Endpoint{},
				Verdict: flow.Verdict_DROPPED,
			},
			checkIsAdvance: true,
			exepectedLabels: []string{
				"reason",
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
			name: "forward source opts with flow",
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
				"reason",
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
			nilObj:     true,
		},
		{
			name: "drop source opts with flow in localcontext",
			opts: &v1alpha1.MetricsContextOptions{
				MetricName:   "drop",
				SourceLabels: []string{"ip", "namespace", "podName", "Workload", "PORT", "serVICE"},
			},
			f: &flow.Flow{
				Source:  &flow.Endpoint{},
				Verdict: flow.Verdict_DROPPED,
			},
			checkIsAdvance: true,
			exepectedLabels: []string{
				"reason",
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
			nilObj:       false,
			localContext: localContext,
		},
		{
			name: "drop source opts with destination flow in localcontext",
			opts: &v1alpha1.MetricsContextOptions{
				MetricName:   "drop",
				SourceLabels: []string{"ip", "namespace", "podName", "Workload", "PORT", "serVICE"},
			},
			f: &flow.Flow{
				Destination: &flow.Endpoint{},
				Verdict:     flow.Verdict_DROPPED,
			},
			checkIsAdvance: true,
			exepectedLabels: []string{
				"reason",
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
			nilObj:       false,
			localContext: localContext,
		},
		{
			name: "drop source opts with source and destination flow in localcontext",
			opts: &v1alpha1.MetricsContextOptions{
				MetricName:   "drop",
				SourceLabels: []string{"ip", "namespace", "podName", "Workload", "PORT", "serVICE"},
			},
			f: &flow.Flow{
				Destination: &flow.Endpoint{},
				Source:      &flow.Endpoint{},
				Verdict:     flow.Verdict_DROPPED,
			},
			checkIsAdvance: true,
			exepectedLabels: []string{
				"reason",
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
			nilObj:       false,
			localContext: localContext,
		},
	}

	for _, tc := range tt {
		for _, metricName := range []string{"drop_count", "drop_bytes"} {
			log.Logger().Info("Running test name", zap.String("name", tc.name), zap.String("metricName", metricName))
			ctrl := gomock.NewController(t)
			f := NewDropCountMetrics(tc.opts, log.Logger(), tc.localContext)
			if tc.nilObj {
				assert.Nil(t, f, "drop metrics should be nil Test Name: %s", tc.name)
				continue
			} else {
				assert.NotNil(t, f, "drp[] metrics should not be nil Test Name: %s", tc.name)
			}
			dropMock := metricsinit.NewMockIGaugeVec(ctrl) //nolint:typecheck

			f.dropMetric = dropMock

			testmetric := prometheus.NewGauge(prometheus.GaugeOpts{
				Name: "testmetric",
				Help: "testmetric",
			})

			dropMock.EXPECT().WithLabelValues(gomock.Any()).Return(testmetric).Times(tc.metricCall)

			assert.Equal(t, f.advEnable, tc.checkIsAdvance, "advance metrics options should be equal Test Name: %s", tc.name)
			assert.Equal(t, tc.exepectedLabels, f.getLabels(), "labels should be equal Test Name: %s", tc.name)

			f.metricName = metricName
			f.ProcessFlow(tc.f)
			ctrl.Finish()
		}
	}
}
