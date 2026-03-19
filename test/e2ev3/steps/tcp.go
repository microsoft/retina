// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package steps

import (
	"context"
	"fmt"
	"log"

	prom "github.com/microsoft/retina/test/e2ev3/framework/prometheus"
)

var (
	tcpStateMetricName            = "networkobservability_tcp_state"
	tcpConnectionRemoteMetricName = "networkobservability_tcp_connection_remote"
)

const (
	stateKey = "state"

	established = "ESTABLISHED"
	listen      = "LISTEN"
	timewait    = "TIME_WAIT"
)

// ValidateRetinaTCPStateStep checks that the TCP state metric exists
// for ESTABLISHED, LISTEN, and TIME_WAIT states.
type ValidateRetinaTCPStateStep struct {
	PortForwardedRetinaPort string
}

func (v *ValidateRetinaTCPStateStep) Do(_ context.Context) error {
	promAddress := fmt.Sprintf("http://localhost:%s/metrics", v.PortForwardedRetinaPort)

	validMetrics := []map[string]string{
		{stateKey: established},
		{stateKey: listen},
		{stateKey: timewait},
	}

	for _, metric := range validMetrics {
		err := prom.CheckMetric(promAddress, tcpStateMetricName, metric)
		if err != nil {
			return fmt.Errorf("failed to verify prometheus metrics: %w", err)
		}
	}

	log.Printf("found metrics matching %+v\n", validMetrics)
	return nil
}

// ValidateRetinaTCPConnectionRemoteStep checks the TCP connection remote metric.
// Currently performs empty validation (no specific labels checked).
type ValidateRetinaTCPConnectionRemoteStep struct {
	PortForwardedRetinaPort string
}

func (v *ValidateRetinaTCPConnectionRemoteStep) Do(_ context.Context) error {
	promAddress := fmt.Sprintf("http://localhost:%s/metrics", v.PortForwardedRetinaPort)

	validMetrics := []map[string]string{}

	for _, metric := range validMetrics {
		err := prom.CheckMetric(promAddress, tcpConnectionRemoteMetricName, metric)
		if err != nil {
			return fmt.Errorf("failed to verify prometheus metrics: %w", err)
		}
	}

	log.Printf("found metrics matching %+v\n", tcpConnectionRemoteMetricName)
	return nil
}
