package latency

import (
	"fmt"
	"log"

	prom "github.com/microsoft/retina/test/e2e/framework/prometheus"
)

var latencyBucketMetricName = "networkobservability_adv_node_apiserver_tcp_handshake_latency"

type ValidateAPIServerLatencyMetric struct {
	PortForwardedRetinaPort string
}

func (v *ValidateAPIServerLatencyMetric) Prevalidate() error {
	return nil
}

func (v *ValidateAPIServerLatencyMetric) Run() error {
	promAddress := fmt.Sprintf("http://localhost:%s/metrics", v.PortForwardedRetinaPort)

	metric := map[string]string{}
	err := prom.CheckMetric(promAddress, latencyBucketMetricName, metric)
	if err != nil {
		return fmt.Errorf("failed to verify prometheus metrics %s: %w", latencyBucketMetricName, err)
	}

	log.Printf("found metrics matching %s\n", latencyBucketMetricName)
	return nil
}

func (v *ValidateAPIServerLatencyMetric) Stop() error {
	return nil
}
