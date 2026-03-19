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
	dropCountMetricName = "networkobservability_drop_count"
	dropBytesMetricName = "networkobservability_drop_bytes"
)

const (
	IPTableRuleDrop = "IPTABLE_RULE_DROP"

	directionKey = "direction"
	reasonKey    = "reason"
)

// ValidateRetinaDropMetricStep checks that drop count and drop bytes metrics
// are present with the expected direction and reason labels.
type ValidateRetinaDropMetricStep struct {
	PortForwardedRetinaPort string
	Direction               string
	Reason                  string
}

func (v *ValidateRetinaDropMetricStep) Do(_ context.Context) error {
	promAddress := fmt.Sprintf("http://localhost:%s/metrics", v.PortForwardedRetinaPort)

	metric := map[string]string{
		directionKey: v.Direction,
		reasonKey:    IPTableRuleDrop,
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
