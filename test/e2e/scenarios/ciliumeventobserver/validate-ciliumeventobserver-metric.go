package ciliumeventobserver

import (
	"fmt"
	"log"

	prom "github.com/microsoft/retina/test/e2e/framework/prometheus"
)

var (
	dropCountMetricName = "hubble_drop_total"
	tcpFlagsMetricName  = "hubble_tcp_flags_total"
	flowsMetricName     = "hubble_flows_processed_total"
)

const (
	destinationKey = "destination"
	sourceKey      = "source"
	protcolKey     = "protocol"
	reasonKey      = "reason"
	directionKey   = "direction"
)

type CEODropMetric struct {
	PortForwardedHubblePort string
	Source                  string
	Protocol                string
	Reason                  string
	Direction               string
}

func (v *CEODropMetric) Run() error {
	promAddress := fmt.Sprintf("http://localhost:%s/metrics", v.PortForwardedHubblePort)

	metric := map[string]string{
		directionKey: v.Direction, reasonKey: v.Reason,
	}

	err := prom.CheckMetric(promAddress, dropCountMetricName, metric)
	if err != nil {
		return fmt.Errorf("failed to verify prometheus metrics %s: %w", dropCountMetricName, err)
	}

	log.Printf("found metrics matching %+v\n", metric)
	return nil
}

func (v *CEODropMetric) Prevalidate() error {
	return nil
}

func (v *CEODropMetric) Stop() error {
	return nil
}

type CEOFlowsMetric struct {
	PortForwardedHubblePort string
	// Source                  string
	// Destination             string
	Protocol string
	Verdict  string
	Type     string
}

// Flows
func (v *CEOFlowsMetric) Run() error {
	promAddress := fmt.Sprintf("http://localhost:%s/metrics", v.PortForwardedHubblePort)

	// Source and Destination are empty for now due to hubble enrichment bug
	// This should be updated in both maps once the bug is fixed
	metric := map[string]string{
		// "destination": v.Destination,
		// "source":      v.Source,
		"protocol": v.Protocol, // TCP, ICMPv6
		"verdict":  v.Verdict,  // FORWARDED
		"type":     v.Type,     // Trace, Drop, PolicyVerdict
	}

	err := prom.CheckMetric(promAddress, flowsMetricName, metric)
	if err != nil {
		return fmt.Errorf("failed to verify prometheus metrics %s: %w", flowsMetricName, err)
	}

	log.Printf("found metrics matching %+v\n", metric)
	return nil
}

func (v *CEOFlowsMetric) Prevalidate() error {
	return nil
}

func (v *CEOFlowsMetric) Stop() error {
	return nil
}

type CEOTCPMetric struct {
	PortForwardedHubblePort string
	// Source string
	// Destination string
	Flag   string
	Family string
}

func (v *CEOTCPMetric) Run() error {
	promAddress := fmt.Sprintf("http://localhost:%s/metrics", v.PortForwardedHubblePort)

	// Source and Destination are empty for now due to hubble enrichment bug
	// This should be updated in both maps once the bug is fixed
	metric := map[string]string{
		// "destination": v.Destination,
		// "source":      v.Source,
		"flag":   v.Flag,
		"family": v.Family,
	}

	err := prom.CheckMetric(promAddress, tcpFlagsMetricName, metric)
	if err != nil {
		return fmt.Errorf("failed to verify prometheus metrics %s: %w", flowsMetricName, err)
	}

	log.Printf("found metrics matching %+v\n", metric)
	return nil
}

func (v *CEOTCPMetric) Prevalidate() error {
	return nil
}

func (v *CEOTCPMetric) Stop() error {
	return nil
}
