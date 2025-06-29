package windows

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	kubernetes "github.com/microsoft/retina/test/e2e/framework/kubernetes"
	prom "github.com/microsoft/retina/test/e2e/framework/prometheus"
)

const (
	// TestExternalIpAddress is the IP address used for testing purposes.
	// It should be a valid external IP address that can be used for testing
	// network observability metrics.
	// This IP address is used in the EventWriter-SetFilter command to generate trace and
	// drop events.
	//Example.com - 23.192.228.84
	TestExternalIpAddress = "23.192.228.84"
)

type ValidateWinBpfMetric struct {
	KubeConfigFilePath        string
	EbpfXdpDeamonSetNamespace string
	EbpfXdpDeamonSetName      string
	RetinaDaemonSetNamespace  string
	RetinaDaemonSetName       string
	NonHpcAppNamespace        string
	NonHpcAppName             string
	NonHpcPodName             string
}

func (v *ValidateWinBpfMetric) GetPromMetrics() (string, error) {
	retinaLabelSelector := "k8s-app=retina"
	var promOutput string
	var err error
	attempts := 10

	for range attempts {
		promOutput, err = kubernetes.ExecCommandInWinPod(
			v.KubeConfigFilePath,
			"C:\\event-writer-helper.bat EventWriter-GetRetinaPromMetrics",
			v.RetinaDaemonSetNamespace,
			retinaLabelSelector,
			false)

		promOutput = prom.StripExecGarbage(promOutput)
		if err == nil && promOutput != "" {
			break
		}
		time.Sleep(3 * time.Second)
	}

	if err != nil {
		return "", err
	}
	return promOutput, nil
}

func (v *ValidateWinBpfMetric) getNonHpcPodIpAddress() (string, error) {

	slog.Info("Executing EventWriter-GetPodIpAddress")
	nonHpcLabelSelector := fmt.Sprintf("app=%s", v.NonHpcAppName)

	slog.Info("Executing  EventWriter-GetPodIpAddress")
	nonHpcIpAddr, err := kubernetes.ExecCommandInWinPod(
		v.KubeConfigFilePath,
		"C:\\event-writer-helper.bat EventWriter-GetPodIpAddress",
		v.NonHpcAppNamespace,
		nonHpcLabelSelector,
		true)

	if err != nil {
		return "", err
	}

	nonHpcIpAddr = strings.TrimSpace(nonHpcIpAddr)

	if strings.Contains(nonHpcIpAddr, "failed") || strings.Contains(nonHpcIpAddr, "error") {
		return "", fmt.Errorf("failed to get nonHpcIpAddr")
	}
	slog.Info("Non HPC IP Addr", "ip", nonHpcIpAddr)

	return nonHpcIpAddr, nil
}

func (v *ValidateWinBpfMetric) getNonHpcPodIfIndex() (string, error) {
	slog.Info("Executing EventWriter-GetPodIfIndex")
	nonHpcLabelSelector := fmt.Sprintf("app=%s", v.NonHpcAppName)

	nonHpcIfIndex, err := kubernetes.ExecCommandInWinPod(
		v.KubeConfigFilePath,
		"C:\\event-writer-helper.bat EventWriter-GetPodIfIndex",
		v.NonHpcAppNamespace,
		nonHpcLabelSelector,
		true)

	if err != nil {
		return "", err
	}

	if strings.Contains(nonHpcIfIndex, "failed") || strings.Contains(nonHpcIfIndex, "error") {
		return "", fmt.Errorf("failed to get nonHpcIfIndex")
	}
	slog.Info("Non HPC Interface Index", "InterfaceIndex", nonHpcIfIndex)

	return nonHpcIfIndex, nil
}

func (v *ValidateWinBpfMetric) attachEventWriter(nonHpcIfIndex string) (string, error) {
	slog.Info("Attaching Event Writer to Non HPC Pod")
	ebpfLabelSelector := fmt.Sprintf("name=%s", v.EbpfXdpDeamonSetName)

	// Attach to the non HPC pod
	output, err := kubernetes.ExecCommandInWinPod(
		v.KubeConfigFilePath,
		fmt.Sprintf("C:\\event-writer-helper.bat EventWriter-Attach %s", nonHpcIfIndex),
		v.EbpfXdpDeamonSetNamespace,
		ebpfLabelSelector,
		true)

	if err != nil {
		return "", err
	}

	if strings.Contains(output, "failed") || strings.Contains(output, "error") || strings.Contains(output, "exiting") {
		return "", fmt.Errorf("failed to attach to non HPC pod interface %s", output)
	}

	return output, nil
}

func (v *ValidateWinBpfMetric) generateTraceEvents() error {

	slog.Info("Generating Trace Events")
	nonHpcLabelSelector := fmt.Sprintf("app=%s", v.NonHpcAppName)
	ebpfLabelSelector := fmt.Sprintf("name=%s", v.EbpfXdpDeamonSetName)

	//TRACE
	output, err := kubernetes.ExecCommandInWinPod(
		v.KubeConfigFilePath,
		fmt.Sprintf("C:\\event-writer-helper.bat EventWriter-SetFilter -event 4 -srcIP %s", TestExternalIpAddress),
		v.EbpfXdpDeamonSetNamespace,
		ebpfLabelSelector,
		true)

	if err != nil {
		return err
	}

	if strings.Contains(output, "failed") || strings.Contains(output, "error") || strings.Contains(output, "exiting") {
		return fmt.Errorf("failed to set filter for event writer")
	}

	numcurls := 10
	for numcurls > 0 {
		_, err = kubernetes.ExecCommandInWinPod(
			v.KubeConfigFilePath,
			fmt.Sprintf("C:\\event-writer-helper.bat EventWriter-Curl %s", TestExternalIpAddress),
			v.NonHpcAppNamespace,
			nonHpcLabelSelector,
			false)
		if err != nil {
			return err
		}
		numcurls--
	}

	return nil
}

func (v *ValidateWinBpfMetric) generateDropEvents() error {
	slog.Info("Generating Drop Events")
	nonHpcLabelSelector := fmt.Sprintf("app=%s", v.NonHpcAppName)
	ebpfLabelSelector := fmt.Sprintf("name=%s", v.EbpfXdpDeamonSetName)

	output, err := kubernetes.ExecCommandInWinPod(
		v.KubeConfigFilePath,
		fmt.Sprintf("C:\\event-writer-helper.bat EventWriter-SetFilter -event 1 -srcIP %s", TestExternalIpAddress),
		v.EbpfXdpDeamonSetNamespace,
		ebpfLabelSelector,
		true)

	if err != nil {
		return err
	}

	if strings.Contains(output, "failed") || strings.Contains(output, "error") || strings.Contains(output, "exiting") {
		return fmt.Errorf("failed to start event writer")
	}

	numcurls := 10
	for numcurls > 0 {
		_, err = kubernetes.ExecCommandInWinPod(
			v.KubeConfigFilePath,
			fmt.Sprintf("C:\\event-writer-helper.bat EventWriter-Curl %s", TestExternalIpAddress),
			v.NonHpcAppNamespace,
			nonHpcLabelSelector,
			false)
		if err != nil {
			return err
		}
		numcurls--
	}

	return nil
}

func (v *ValidateWinBpfMetric) verifyBasicMetrics(promOutput string) error {

	var fwdBytes float64 = 0
	var drpBytes float64 = 0
	var fwdCount float64 = 0
	var drpCount float64 = 0

	fwd_labels := map[string]string{
		"direction": "ingress",
	}

	drp_labels := map[string]string{
		"direction": "ingress",
		"reason":    "130, 0",
	}

	if promOutput == "" {
		slog.Info("No Prometheus metrics found, skipping validation")
	} else {
		err := prom.CheckMetricFromBuffer([]byte(promOutput), "networkobservability_forward_bytes", fwd_labels)
		if err != nil {
			return fmt.Errorf("failed to verify prometheus metrics: %w", err)
		}

		fwdBytes, err = prom.GetMetricGuageValueFromBuffer([]byte(promOutput), "networkobservability_forward_bytes", fwd_labels)
		if err != nil && strings.Contains(err.Error(), "failed to parse prometheus metrics") {
			return err
		}
		slog.Info("networkobservability_forward_bytes value", "value", fwdBytes, "labels", fwd_labels)

		fwdCount, err = prom.GetMetricGuageValueFromBuffer([]byte(promOutput), "networkobservability_forward_count", fwd_labels)
		if err != nil && strings.Contains(err.Error(), "failed to parse prometheus metrics") {
			return err
		}
		slog.Info("networkobservability_forward_count value", "value", fwdCount, "labels", fwd_labels)

		drpBytes, err = prom.GetMetricGuageValueFromBuffer([]byte(promOutput), "networkobservability_drop_bytes", drp_labels)
		if err != nil && strings.Contains(err.Error(), "failed to parse prometheus metrics") {
			return err
		}
		slog.Info("networkobservability_drop_bytes value", "value", drpBytes, "labels", drp_labels)

		drpCount, err = prom.GetMetricGuageValueFromBuffer([]byte(promOutput), "networkobservability_drop_count", drp_labels)
		if err != nil && strings.Contains(err.Error(), "failed to parse prometheus metrics") {
			return err
		}
		slog.Info("networkobservability_drop_count value", "value", drpCount, "labels", drp_labels)
	}

	return nil

}

func (v *ValidateWinBpfMetric) verifyAdvancedMetrics(nonHpcIpAddr, promOutput string) error {

	// Advanced Metrics
	adv_fwd_count_labels := map[string]string{
		"direction":     "egress",
		"ip":            "23.192.228.84",
		"namespace":     "",
		"podname":       "",
		"workload_kind": "unknown",
		"workload_name": "unknown",
	}
	err := prom.CheckMetricFromBuffer([]byte(promOutput), "networkobservability_adv_forward_count", adv_fwd_count_labels)
	if err != nil {
		return fmt.Errorf("failed to find networkobservability_adv_forward_count")
	}

	tcpFlags := []string{"ACK", "FIN", "PSH"}
	for _, flag := range tcpFlags {
		tcpFlagLabels := map[string]string{
			"flag":          flag,
			"ip":            "23.192.228.84",
			"namespace":     "",
			"podname":       "",
			"workload_kind": "unknown",
			"workload_name": "unknown",
		}

		err = prom.CheckMetricFromBuffer([]byte(promOutput), "networkobservability_adv_tcpflags_count", tcpFlagLabels)
		if err != nil {
			return fmt.Errorf("failed to find networkobservability_adv_tcpflags_count for flag %s: %w", flag, err)
		}
		slog.Info("Found TCP flag metric", "flag", flag)
	}

	adv_drop_byte_labels := map[string]string{
		"direction":     "egress",
		"ip":            "23.192.228.84",
		"namespace":     "",
		"podname":       "",
		"reason":        "Drop_NotAccepted",
		"workload_kind": "unknown",
		"workload_name": "unknown",
	}
	err = prom.CheckMetricFromBuffer([]byte(promOutput), "networkobservability_adv_drop_bytes", adv_drop_byte_labels)
	if err != nil {
		return fmt.Errorf("failed to find networkobservability_adv_drop_bytes")
	}

	adv_drop_count_labels := map[string]string{
		"direction":     "egress",
		"ip":            "23.192.228.84",
		"namespace":     "",
		"podname":       "",
		"reason":        "Drop_NotAccepted",
		"workload_kind": "unknown",
		"workload_name": "unknown",
	}
	err = prom.CheckMetricFromBuffer([]byte(promOutput), "networkobservability_adv_drop_count", adv_drop_count_labels)
	if err != nil {
		return fmt.Errorf("failed to find networkobservability_adv_drop_count")
	}

	adv_fwd_count_labels = map[string]string{
		"direction":     "ingress",
		"ip":            nonHpcIpAddr,
		"namespace":     v.NonHpcAppNamespace,
		"podname":       v.NonHpcPodName,
		"workload_kind": "unknown",
		"workload_name": "unknown",
	}
	err = prom.CheckMetricFromBuffer([]byte(promOutput), "networkobservability_adv_forward_count", adv_fwd_count_labels)
	if err != nil {
		return fmt.Errorf("failed to find networkobservability_adv_forward_count")
	}

	for _, flag := range tcpFlags {
		tcpFlagLabels := map[string]string{
			"flag":          flag,
			"ip":            nonHpcIpAddr,
			"namespace":     v.NonHpcAppNamespace,
			"podname":       v.NonHpcPodName,
			"workload_kind": "unknown",
			"workload_name": "unknown",
		}

		err = prom.CheckMetricFromBuffer([]byte(promOutput), "networkobservability_adv_tcpflags_count", tcpFlagLabels)
		if err != nil {
			return fmt.Errorf("failed to find networkobservability_adv_tcpflags_count for flag %s: %w", flag, err)
		}
		slog.Info("Found TCP flag metric", "flag", flag)
	}

	adv_drop_byte_labels = map[string]string{
		"direction":     "ingress",
		"ip":            nonHpcIpAddr,
		"namespace":     v.NonHpcAppNamespace,
		"podname":       v.NonHpcPodName,
		"reason":        "Drop_NotAccepted",
		"workload_kind": "unknown",
		"workload_name": "unknown",
	}
	err = prom.CheckMetricFromBuffer([]byte(promOutput), "networkobservability_adv_drop_bytes", adv_drop_byte_labels)
	if err != nil {
		return fmt.Errorf("failed to find networkobservability_adv_drop_bytes with ingress label")
	}

	adv_drop_count_labels = map[string]string{
		"direction":     "ingress",
		"ip":            nonHpcIpAddr,
		"namespace":     v.NonHpcAppNamespace,
		"podname":       v.NonHpcPodName,
		"reason":        "Drop_NotAccepted",
		"workload_kind": "unknown",
		"workload_name": "unknown",
	}
	err = prom.CheckMetricFromBuffer([]byte(promOutput), "networkobservability_adv_drop_count", adv_drop_count_labels)
	if err != nil {
		return fmt.Errorf("failed to find networkobservability_adv_drop_count with ingress label")
	}
	return nil
}

func (v *ValidateWinBpfMetric) Run() error {

	nonHpcLabelSelector := fmt.Sprintf("app=%s", v.NonHpcAppName)
	slog.Info("Waiting for Non HPC Pod to come up")
	// Wait for the non HPC pod to be ready. Maximum wait time is 15 minutes in case the Pods are very slow to come up.
	kubernetes.WaitForPodReadyWithTimeOut(context.TODO(), v.KubeConfigFilePath, v.NonHpcAppNamespace, nonHpcLabelSelector, 15*time.Minute)
	slog.Info("Non HPC Pod is ready")

	nonHpcIpAddr, err := v.getNonHpcPodIpAddress()

	if err != nil {
		return err
	}

	nonHpcIfIndex, err := v.getNonHpcPodIfIndex()

	if err != nil {
		return err
	}

	// Attach to the non HPC pod
	_, err = v.attachEventWriter(nonHpcIfIndex)

	if err != nil {
		return err
	}

	// Generate trace events
	err = v.generateTraceEvents()

	if err != nil {
		return err
	}

	// generate drop events
	err = v.generateDropEvents()

	if err != nil {
		return err
	}

	slog.Info("Waiting for basic metrics to be updated as part of next polling cycle")
	time.Sleep(12 * time.Second)
	promOutput, err := v.GetPromMetrics()

	if err != nil {
		return err
	}

	slog.Info("Prometheus metrics output", "output", promOutput)

	err = v.verifyBasicMetrics(promOutput)
	if err != nil {

		return fmt.Errorf("failed to verify basic metrics: %w", err)
	}
	slog.Info("Basic metrics verified successfully")

	err = v.verifyAdvancedMetrics(nonHpcIpAddr, promOutput)
	if err != nil {
		return fmt.Errorf("failed to verify advanced metrics: %w", err)
	}
	slog.Info("Advanced metrics verified successfully")

	return nil
}

func (v *ValidateWinBpfMetric) Prevalidate() error {
	return nil
}

func (v *ValidateWinBpfMetric) Stop() error {
	return nil
}
