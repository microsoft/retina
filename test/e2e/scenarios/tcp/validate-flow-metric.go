package flow

import (
	"fmt"
	"log"

	prom "github.com/microsoft/retina/test/e2e/framework/prometheus"
	"github.com/microsoft/retina/test/e2e/framework/types"
)

var tcpStateMetricName = "networkobservability_tcp_state"

const (
	state = "state"

	established = "ESTABLISHED"
	listen      = "LISTEN"
	timewait    = "TIME_WAIT"
)

type ValidateRetinaTCPStateMetric struct {
	PortForwardedRetinaPort string
}

func (v *ValidateRetinaTCPStateMetric) Run(_ *types.RuntimeObjects) error {
	promAddress := fmt.Sprintf("http://localhost:%s/metrics", v.PortForwardedRetinaPort)

	validMetrics := []map[string]string{
		{state: established},
		{state: listen},
		{state: timewait},
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

func (v *ValidateRetinaTCPStateMetric) PreRun() error {
	return nil
}

func (v *ValidateRetinaTCPStateMetric) Stop() error {
	return nil
}
