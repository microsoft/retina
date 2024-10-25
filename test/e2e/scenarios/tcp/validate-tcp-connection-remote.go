package flow

import (
	"fmt"
	"log"

	prom "github.com/microsoft/retina/test/e2e/framework/prometheus"
)

var tcpConnectionRemoteMetricName = "networkobservability_tcp_connection_remote"

const (
	address = "address"
	port    = "port"
)

type ValidateRetinaTCPConnectionRemoteMetric struct {
	PortForwardedRetinaPort string
}

func (v *ValidateRetinaTCPConnectionRemoteMetric) Run() error {
	promAddress := fmt.Sprintf("http://localhost:%s/metrics", v.PortForwardedRetinaPort)

	validMetrics := []map[string]string{}

	for _, metric := range validMetrics {
		err := prom.CheckMetric(promAddress, tcpConnectionRemoteMetricName, metric)
		if err != nil {
			return fmt.Errorf("failed to verify prometheus metrics: %w", err)
		}
	}

	log.Printf("found metrics matching %+v\n", validMetrics)
	return nil
}

func (v *ValidateRetinaTCPConnectionRemoteMetric) Prevalidate() error {
	return nil
}

func (v *ValidateRetinaTCPConnectionRemoteMetric) Stop() error {
	return nil
}
