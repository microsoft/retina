// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package metrics

import (
	"testing"

	"github.com/cilium/cilium/api/v1/flow"
	"github.com/stretchr/testify/assert"
)

type TestCtxOpts struct {
	name         string
	opts         []string
	ctxType      ctxOptionType
	expected     []string
	f            *flow.Flow
	expectedVals []string
}

func TestNewCtxOps(t *testing.T) {
	tt := []TestCtxOpts{
		{
			name:         "empty opts",
			opts:         []string{},
			ctxType:      source,
			expected:     []string{},
			f:            &flow.Flow{},
			expectedVals: []string{},
		},
		{
			name:    "source opts 1",
			opts:    []string{"ip", "namespace", "podName", "Workload", "PORT", "serVICE", "zone"},
			ctxType: source,
			expected: []string{
				"source_ip", "source_namespace", "source_podname",
				"source_workload_kind", "source_workload_name", "source_service", "source_port",
				"source_zone",
			},
			f:            &flow.Flow{},
			expectedVals: []string{"unknown", "unknown", "unknown", "unknown", "unknown", "unknown", "unknown", "unknown"},
		},
		{
			name:    "dest opts 1",
			opts:    []string{"ip", "namespace", "podName", "Workload", "PORT", "serVICE", "zone"},
			ctxType: destination,
			expected: []string{
				"destination_ip", "destination_namespace", "destination_podname",
				"destination_workload_kind", "destination_workload_name", "destination_service", "destination_port",
				"destination_zone",
			},
			f:            &flow.Flow{},
			expectedVals: []string{"unknown", "unknown", "unknown", "unknown", "unknown", "unknown", "unknown", "unknown"},
		},
		{
			name:    "source opts with flow",
			opts:    []string{"ip", "namespace", "podName", "Workload", "PORT", "serVICE", "zone"},
			ctxType: source,
			expected: []string{
				"source_ip", "source_namespace", "source_podname",
				"source_workload_kind", "source_workload_name", "source_service", "source_port",
				"source_zone",
			},
			f: &flow.Flow{
				Source: &flow.Endpoint{
					Namespace: "ns",
					PodName:   "test",
				},
			},
			expectedVals: []string{"unknown", "ns", "test", "unknown", "unknown", "unknown", "unknown", "unknown"},
		},
		{
			name:    "dest opts with flow",
			opts:    []string{"ip", "namespace", "podName", "Workload", "PORT", "serVICE", "zone"},
			ctxType: destination,
			expected: []string{
				"destination_ip", "destination_namespace", "destination_podname",
				"destination_workload_kind", "destination_workload_name", "destination_service", "destination_port",
				"destination_zone",
			},
			f: &flow.Flow{
				Destination: &flow.Endpoint{
					Namespace: "ns",
					PodName:   "test",
				},
			},
			expectedVals: []string{"unknown", "ns", "test", "unknown", "unknown", "unknown", "unknown", "unknown"},
		},
		{
			name:     "source opts of ip",
			opts:     []string{"ip", "namespace", "podName"},
			ctxType:  source,
			expected: []string{"source_ip", "source_namespace", "source_podname"},
			f: &flow.Flow{
				Source: &flow.Endpoint{
					Namespace: "ns",
					PodName:   "test",
				},
				IP: &flow.IP{
					Source: "10.0.0.1",
				},
			},
			expectedVals: []string{"10.0.0.1", "ns", "test"},
		},
		{
			name:     "dest opts of ip",
			opts:     []string{"ip", "namespace", "podName"},
			ctxType:  destination,
			expected: []string{"destination_ip", "destination_namespace", "destination_podname"},
			f: &flow.Flow{
				Destination: &flow.Endpoint{
					Namespace: "ns",
					PodName:   "test",
				},
				IP: &flow.IP{
					Destination: "10.0.0.1",
				},
			},
			expectedVals: []string{"10.0.0.1", "ns", "test"},
		},
		{
			name:     "dest opts of ip with no destination info",
			opts:     []string{"ip", "namespace", "podName"},
			ctxType:  destination,
			expected: []string{"destination_ip", "destination_namespace", "destination_podname"},
			f: &flow.Flow{
				Source: &flow.Endpoint{
					Namespace: "ns",
					PodName:   "test",
				},
				IP: &flow.IP{
					Source: "10.0.0.1",
				},
			},
			expectedVals: []string{"", "unknown", "unknown"},
		},
	}

	for _, tc := range tt {
		c := NewCtxOption(tc.opts, tc.ctxType)
		assert.Equal(t, tc.expected, c.getLabels(), "labels should match %s", tc.name)
		values := c.getValues(tc.f)
		assert.Equal(t, tc.expectedVals, values, "values should match %s", tc.name)
	}
}
