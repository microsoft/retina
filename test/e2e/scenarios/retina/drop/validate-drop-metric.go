//go:build e2eframework

package drop

import (
	"fmt"
	"log"

	prom "github.com/microsoft/retina/test/e2e/framework/prometheus"
)

var (
	dropCountMetricName = "networkobservability_drop_count"
	dropBytesMetricName = "networkobservability_drop_bytes"
)

const (
	destinationKey = "destination"
	sourceKey      = "source"
	protcolKey     = "protocol"
	reasonKey      = "reason"
	directionKey   = "direction"
)

type ValidateRetinaDropMetric struct {
	PortForwardedRetinaPort string
	Source                  string
	Protocol                string
	Reason                  string
	Direction               string
}

func (v *ValidateRetinaDropMetric) Run() error {
	promAddress := fmt.Sprintf("http://localhost:%s/metrics", v.PortForwardedRetinaPort)

	metric := map[string]string{
		directionKey: v.Direction, reasonKey: IPTableRuleDrop,
	}

	err := prom.CheckMetric(promAddress, dropCountMetricName, metric)
	if err != nil {
		return fmt.Errorf("failed to verify prometheus metrics %s: %w", dropCountMetricName, err)
	}

	err = prom.CheckMetric(promAddress, dropBytesMetricName, metric)
	if err != nil {
		return fmt.Errorf("failed to verify prometheus metrics %s: %w", dropBytesMetricName, err)
	}

	log.Printf("found metrics matching %+v\n", metric)
	return nil
}

func (v *ValidateRetinaDropMetric) Prevalidate() error {
	return nil
}

func (v *ValidateRetinaDropMetric) Stop() error {
	return nil
}
