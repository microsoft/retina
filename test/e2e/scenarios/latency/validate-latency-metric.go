package latency

import (
	"fmt"
	"log"

	"github.com/microsoft/retina/test/e2e/framework/constants"
	prom "github.com/microsoft/retina/test/e2e/framework/prometheus"
	"github.com/pkg/errors"
)

var latencyBucketMetricName = "networkobservability_adv_node_apiserver_tcp_handshake_latency"

type ValidateAPIServerLatencyMetric struct{}

func (v *ValidateAPIServerLatencyMetric) Prevalidate() error {
	return nil
}

func (v *ValidateAPIServerLatencyMetric) Run() error {
	promAddress := fmt.Sprintf("http://localhost:%s/metrics", constants.RetinaMetricsPort)

	metric := map[string]string{}
	err := prom.CheckMetric(promAddress, latencyBucketMetricName, metric)
	if err != nil {
		return errors.Wrapf(err, "failed to verify latency metrics %s", latencyBucketMetricName)
	}

	log.Printf("found metrics matching %s\n", latencyBucketMetricName)
	return nil
}

func (v *ValidateAPIServerLatencyMetric) Stop() error {
	return nil
}
