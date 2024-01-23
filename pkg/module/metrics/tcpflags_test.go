// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
//
//nolint:all
package metrics

import (
	"testing"

	"github.com/microsoft/retina/crd/api/v1alpha1"
	"github.com/microsoft/retina/pkg/log"
	metricsinit "github.com/microsoft/retina/pkg/metrics"
	"github.com/cilium/cilium/api/v1/flow"
	"github.com/golang/mock/gomock"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestNewTCPMetrics(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())

	tt := []TestMetrics{
		{
			name:            "empty opts",
			opts:            &v1alpha1.MetricsContextOptions{},
			checkIsAdvance:  false,
			f:               &flow.Flow{},
			exepectedLabels: []string{},
			metricCall:      1,
			nilObj:          true,
		},
		{
			name: "empty opts nil flow",
			opts: &v1alpha1.MetricsContextOptions{
				MetricName: "tcpflags",
			},
			checkIsAdvance:  false,
			f:               nil,
			exepectedLabels: []string{"flag"},
			metricCall:      0,
		},
		{
			name: "plain opts",
			opts: &v1alpha1.MetricsContextOptions{
				MetricName: "tcpflags",
			},
			checkIsAdvance:  false,
			f:               &flow.Flow{},
			exepectedLabels: []string{"flag"},
			metricCall:      0,
		},
		{
			name: "source opts 1 without metric name",
			opts: &v1alpha1.MetricsContextOptions{
				SourceLabels: []string{"ip", "namespace", "podName", "Workload", "PORT", "serVICE"},
			},
			f: &flow.Flow{
				Verdict: flow.Verdict_FORWARDED,
			},
			exepectedLabels: []string{
				"flag",
			},
			metricCall: 0,
			nilObj:     true,
		},
		{
			name: "source opts 1",
			opts: &v1alpha1.MetricsContextOptions{
				MetricName:   "flag",
				SourceLabels: []string{"ip", "namespace", "podName", "Workload", "PORT", "serVICE"},
			},
			f:              &flow.Flow{},
			checkIsAdvance: true,
			exepectedLabels: []string{
				"flag",
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
			name: "dest opts 1",
			opts: &v1alpha1.MetricsContextOptions{
				MetricName:        "flag",
				DestinationLabels: []string{"ip", "namespace", "podName", "Workload", "PORT", "serVICE"},
			},
			f: &flow.Flow{
				Verdict: flow.Verdict_FORWARDED,
			},
			checkIsAdvance: true,
			exepectedLabels: []string{
				"flag",
				"destination_ip",
				"destination_namespace",
				"destination_podname",
				"destination_workloadKind",
				"destination_workloadName",
				"destination_service",
				"destination_port",
			},
			metricCall: 0,
		},
		{
			name: "source opts with flow",
			opts: &v1alpha1.MetricsContextOptions{
				MetricName:   "flag",
				SourceLabels: []string{"ip", "namespace", "podName", "Workload", "PORT", "serVICE"},
			},
			f: &flow.Flow{
				Source:  &flow.Endpoint{},
				Verdict: flow.Verdict_FORWARDED,
			},
			checkIsAdvance: true,
			exepectedLabels: []string{
				"flag",
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
			name: "source opts with flow with flags",
			opts: &v1alpha1.MetricsContextOptions{
				MetricName:   "flag",
				SourceLabels: []string{"ip", "namespace", "podName", "Workload", "PORT", "serVICE"},
			},
			f: &flow.Flow{
				Source:  &flow.Endpoint{},
				Verdict: flow.Verdict_FORWARDED,
				L4: &flow.Layer4{
					Protocol: &flow.Layer4_TCP{
						TCP: &flow.TCP{
							Flags: &flow.TCPFlags{
								SYN: true,
							},
						},
					},
				},
			},
			checkIsAdvance: true,
			exepectedLabels: []string{
				"flag",
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
			name: "source opts with nil flow",
			opts: &v1alpha1.MetricsContextOptions{
				MetricName:   "flag",
				SourceLabels: []string{"ip", "namespace", "podName", "Workload", "PORT", "serVICE"},
			},
			f: &flow.Flow{
				Source:  &flow.Endpoint{},
				Verdict: flow.Verdict_FORWARDED,
				L4: &flow.Layer4{
					Protocol: &flow.Layer4_TCP{
						TCP: &flow.TCP{
							Flags: nil,
						},
					},
				},
			},
			checkIsAdvance: true,
			exepectedLabels: []string{
				"flag",
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
			name: "source opts with flow with all flags except ack",
			opts: &v1alpha1.MetricsContextOptions{
				MetricName:   "flag",
				SourceLabels: []string{"ip", "namespace", "podName", "Workload", "PORT", "serVICE"},
			},
			f: &flow.Flow{
				Source:  &flow.Endpoint{},
				Verdict: flow.Verdict_FORWARDED,
				L4: &flow.Layer4{
					Protocol: &flow.Layer4_TCP{
						TCP: &flow.TCP{
							Flags: &flow.TCPFlags{
								SYN: true,
								FIN: true,
								RST: true,
								PSH: true,
								URG: true,
								ECE: true,
								CWR: true,
							},
						},
					},
				},
			},
			checkIsAdvance: true,
			exepectedLabels: []string{
				"flag",
				"source_ip",
				"source_namespace",
				"source_podname",
				"source_workloadKind",
				"source_workloadName",
				"source_service",
				"source_port",
			},
			metricCall: 7,
		},
		{
			name: "dest opts with flow with all flags ",
			opts: &v1alpha1.MetricsContextOptions{
				MetricName:        "flag",
				DestinationLabels: []string{"ip", "namespace", "podName", "Workload", "PORT", "serVICE"},
			},
			f: &flow.Flow{
				Source:  &flow.Endpoint{},
				Verdict: flow.Verdict_FORWARDED,
				L4: &flow.Layer4{
					Protocol: &flow.Layer4_TCP{
						TCP: &flow.TCP{
							Flags: &flow.TCPFlags{
								SYN: true,
								FIN: true,
								RST: true,
								PSH: true,
								URG: true,
								ECE: true,
								CWR: true,
								ACK: true,
							},
						},
					},
				},
			},
			checkIsAdvance: true,
			exepectedLabels: []string{
				"flag",
				"destination_ip",
				"destination_namespace",
				"destination_podname",
				"destination_workloadKind",
				"destination_workloadName",
				"destination_service",
				"destination_port",
			},
			metricCall: 7,
		},
		{
			name: "dest opts with flow with all but syn flags ",
			opts: &v1alpha1.MetricsContextOptions{
				MetricName:        "flag",
				DestinationLabels: []string{"ip", "namespace", "podName", "Workload", "PORT", "serVICE"},
			},
			f: &flow.Flow{
				Source:  &flow.Endpoint{},
				Verdict: flow.Verdict_FORWARDED,
				L4: &flow.Layer4{
					Protocol: &flow.Layer4_TCP{
						TCP: &flow.TCP{
							Flags: &flow.TCPFlags{
								FIN: true,
								RST: true,
								PSH: true,
								URG: true,
								ECE: true,
								CWR: true,
								ACK: true,
							},
						},
					},
				},
			},
			checkIsAdvance: true,
			exepectedLabels: []string{
				"flag",
				"destination_ip",
				"destination_namespace",
				"destination_podname",
				"destination_workloadKind",
				"destination_workloadName",
				"destination_service",
				"destination_port",
			},
			metricCall: 7,
		},
		{
			name: "dest opts with flow with all flags dropped verdict",
			opts: &v1alpha1.MetricsContextOptions{
				MetricName:        "flag",
				DestinationLabels: []string{"ip", "namespace", "podName", "Workload", "PORT", "serVICE"},
			},
			f: &flow.Flow{
				Source:  &flow.Endpoint{},
				Verdict: flow.Verdict_DROPPED,
				L4: &flow.Layer4{
					Protocol: &flow.Layer4_TCP{
						TCP: &flow.TCP{
							Flags: &flow.TCPFlags{
								SYN: true,
								FIN: true,
								RST: true,
								PSH: true,
								URG: true,
								ECE: true,
								CWR: true,
								ACK: true,
							},
						},
					},
				},
			},
			checkIsAdvance: true,
			exepectedLabels: []string{
				"flag",
				"destination_ip",
				"destination_namespace",
				"destination_podname",
				"destination_workloadKind",
				"destination_workloadName",
				"destination_service",
				"destination_port",
			},
			metricCall: 0,
		},
		{
			name: "local ctx dest opts with flow with all flags ",
			opts: &v1alpha1.MetricsContextOptions{
				MetricName:   "flag",
				SourceLabels: []string{"ip", "namespace", "podName", "Workload", "PORT", "serVICE"},
			},
			f: &flow.Flow{
				Source:  &flow.Endpoint{},
				Verdict: flow.Verdict_FORWARDED,
				L4: &flow.Layer4{
					Protocol: &flow.Layer4_TCP{
						TCP: &flow.TCP{
							Flags: &flow.TCPFlags{
								SYN: true,
								FIN: true,
								RST: true,
								PSH: true,
								URG: true,
								ECE: true,
								CWR: true,
								ACK: true,
							},
						},
					},
				},
			},
			checkIsAdvance: true,
			exepectedLabels: []string{
				"flag",
				"ip",
				"namespace",
				"podname",
				"workloadKind",
				"workloadName",
				"service",
				"port",
			},
			localContext: localContext,
			metricCall:   7,
		},
		{
			name: "local ctx dest opts with flow with all flags ",
			opts: &v1alpha1.MetricsContextOptions{
				MetricName:   "flag",
				SourceLabels: []string{"ip", "namespace", "podName", "Workload", "PORT", "serVICE"},
			},
			f: &flow.Flow{
				Verdict: flow.Verdict_FORWARDED,
				L4: &flow.Layer4{
					Protocol: &flow.Layer4_TCP{
						TCP: &flow.TCP{
							Flags: &flow.TCPFlags{
								SYN: true,
								FIN: true,
								RST: true,
								PSH: true,
								URG: true,
								ECE: true,
								CWR: true,
								ACK: true,
							},
						},
					},
				},
			},
			checkIsAdvance: true,
			exepectedLabels: []string{
				"flag",
				"ip",
				"namespace",
				"podname",
				"workloadKind",
				"workloadName",
				"service",
				"port",
			},
			localContext: localContext,
			metricCall:   0,
		},
		{
			name: "local ctx ars and dest opts with flow with all flags ",
			opts: &v1alpha1.MetricsContextOptions{
				MetricName:   "flag",
				SourceLabels: []string{"ip", "namespace", "podName", "Workload", "PORT", "serVICE"},
			},
			f: &flow.Flow{
				Source:      &flow.Endpoint{},
				Destination: &flow.Endpoint{},
				Verdict:     flow.Verdict_FORWARDED,
				L4: &flow.Layer4{
					Protocol: &flow.Layer4_TCP{
						TCP: &flow.TCP{
							Flags: &flow.TCPFlags{
								SYN: true,
								FIN: true,
								RST: true,
								PSH: true,
								URG: true,
								ECE: true,
								CWR: true,
								ACK: true,
							},
						},
					},
				},
			},
			checkIsAdvance: true,
			exepectedLabels: []string{
				"flag",
				"ip",
				"namespace",
				"podname",
				"workloadKind",
				"workloadName",
				"service",
				"port",
			},
			localContext: localContext,
			metricCall:   14,
		},
	}

	for _, tc := range tt {
		log.Logger().Info("Running test name", zap.String("name", tc.name))
		ctrl := gomock.NewController(t)

		tcp := NewTCPMetrics(tc.opts, log.Logger(), tc.localContext)
		if tc.nilObj {
			assert.Nil(t, tcp, "forward metrics should be nil Test Name: %s", tc.name)
			continue
		} else {
			assert.NotNil(t, tcp, "forward metrics should not be nil Test Name: %s", tc.name)
		}

		tcpFlagMockMetrics := metricsinit.NewMockIGaugeVec(ctrl) //nolint:staticcheck
		tcp.tcpFlagsMetrics = tcpFlagMockMetrics

		testmetric := prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "testmetric",
			Help: "testmetric",
		})

		tcpFlagMockMetrics.EXPECT().WithLabelValues(gomock.Any()).Return(testmetric).Times(tc.metricCall)
		assert.Equal(t, tc.checkIsAdvance, tcp.advEnable, "IsAdvance should be %v Test Name: %s", tc.checkIsAdvance, tc.name)
		assert.Equal(t, tc.exepectedLabels, tcp.getLabels(), "labels should be %v Test Name: %s", tc.exepectedLabels, tc.name)

		tcp.ProcessFlow(tc.f)
		ctrl.Finish()
	}
}
