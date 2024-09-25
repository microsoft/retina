package latency

import (
	"fmt"
	"log"

	"github.com/microsoft/retina/test/e2e/common"
	prom "github.com/microsoft/retina/test/e2e/framework/prometheus"
	"github.com/microsoft/retina/test/e2e/framework/types"
	"github.com/pkg/errors"
)

var latencyBucketMetricName = "networkobservability_adv_node_apiserver_tcp_handshake_latency"

type ValidateAPIServerLatencyMetric struct{}

func (v *ValidateAPIServerLatencyMetric) PreRun() error {
	return nil
}

func (v *ValidateAPIServerLatencyMetric) Run(_ *types.RuntimeObjects) error {
	promAddress := fmt.Sprintf("http://localhost:%d/metrics", common.RetinaPort)

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
