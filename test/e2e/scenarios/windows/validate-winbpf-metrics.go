package windows

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	kubernetes "github.com/microsoft/retina/test/e2e/framework/kubernetes"
	prom "github.com/microsoft/retina/test/e2e/framework/prometheus"
)

var (
	// ErrForwardBytesZero indicates forward bytes metric is zero
	ErrForwardBytesZero = errors.New("forward bytes metric is zero, expected non-zero value")
	// ErrForwardCountZero indicates forward count metric is zero
	ErrForwardCountZero = errors.New("forward count metric is zero, expected non-zero value")
	// ErrDropBytesZero indicates drop bytes metric is zero
	ErrDropBytesZero = errors.New("drop bytes metric is zero, expected non-zero value")
	// ErrDropCountZero indicates drop count metric is zero
	ErrDropCountZero = errors.New("drop count metric is zero, expected non-zero value")
	// ErrWindowsDropBytesZero indicates windows drop bytes metric is zero
	ErrWindowsDropBytesZero = errors.New("windows drop bytes metric is zero, expected non-zero value")
	// ErrWindowsDropCountZero indicates windows drop count metric is zero
	ErrWindowsDropCountZero = errors.New("windows drop count metric is zero, expected non-zero value")
	ErrGetNonHpcIPAddr      = errors.New("failed to get nonHpcIPAddr")
	ErrGetNonHpcIfIndex     = errors.New("failed to get nonHpcIfIndex")
	ErrAttachInterface      = errors.New("failed to attach to non HPC pod interface")
	ErrSetFilterEventWriter = errors.New("failed to set filter for event writer")
	ErrStartEventWriter     = errors.New("failed to start event writer")
)

const (
	// TestExternalIPAddress is the IP address used for testing purposes.
	// It should be a valid external IP address that can be used for testing
	// network observability metrics.
	// This IP address is used in the EventWriter-SetFilter command to generate trace and
	// drop events.
	// Example.com - 23.192.228.84
	TestExternalIPAddress = "23.192.228.84"
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
			false,
		)

		promOutput = prom.StripExecGarbage(promOutput)
		if err == nil && promOutput != "" {
			break
		}
		time.Sleep(3 * time.Second)
	}

	if err != nil {
		return "", fmt.Errorf("executing EventWriter command: %w", err)
	}
	return promOutput, nil
}

func (v *ValidateWinBpfMetric) getNonHpcPodIPAddress() (string, error) {
	slog.Info("Executing EventWriter-GetPodIpAddress")
	nonHpcLabelSelector := "app=" + v.NonHpcAppName

	nonHpcIPAddr, err := kubernetes.ExecCommandInWinPod(
		v.KubeConfigFilePath,
		"C:\\event-writer-helper.bat EventWriter-GetPodIpAddress",
		v.NonHpcAppNamespace,
		nonHpcLabelSelector,
		true,
	)
	if err != nil {
		return "", fmt.Errorf("executing EventWriter command: %w", err)
	}
	nonHpcIPAddr = strings.TrimSpace(nonHpcIPAddr)

	if strings.Contains(nonHpcIPAddr, "failed") || strings.Contains(nonHpcIPAddr, "error") {
		return "", ErrGetNonHpcIPAddr
	}
	slog.Info("Non HPC IP Addr", "ip", nonHpcIPAddr)

	return nonHpcIPAddr, nil
}

func (v *ValidateWinBpfMetric) getNonHpcPodIfIndex() (string, error) {
	slog.Info("Executing EventWriter-GetPodIfIndex")
	nonHpcLabelSelector := "app=" + v.NonHpcAppName

	nonHpcIfIndex, err := kubernetes.ExecCommandInWinPod(
		v.KubeConfigFilePath,
		"C:\\event-writer-helper.bat EventWriter-GetPodIfIndex",
		v.NonHpcAppNamespace,
		nonHpcLabelSelector,
		true,
	)
	if err != nil {
		return "", fmt.Errorf("executing EventWriter command: %w", err)
	}

	if strings.Contains(nonHpcIfIndex, "failed") || strings.Contains(nonHpcIfIndex, "error") {
		return "", ErrGetNonHpcIfIndex
	}
	slog.Info("Non HPC Interface Index", "InterfaceIndex", nonHpcIfIndex)

	return nonHpcIfIndex, nil
}

func (v *ValidateWinBpfMetric) attachEventWriter(nonHpcIfIndex string) (string, error) {
	slog.Info("Attaching Event Writer to Non HPC Pod")
	ebpfLabelSelector := "name=" + v.EbpfXdpDeamonSetName

	// Attach to the non HPC pod
	output, err := kubernetes.ExecCommandInWinPod(
		v.KubeConfigFilePath,
		"C:\\event-writer-helper.bat EventWriter-Attach "+nonHpcIfIndex,
		v.EbpfXdpDeamonSetNamespace,
		ebpfLabelSelector,
		true,
	)
	if err != nil {
		return "", fmt.Errorf("executing EventWriter command: %w", err)
	}

	if strings.Contains(output, "failed") || strings.Contains(output, "error") || strings.Contains(output, "exiting") {
		return "", fmt.Errorf("%w: %s", ErrAttachInterface, output)
	}

	return output, nil
}

func (v *ValidateWinBpfMetric) generateTraceEvents() error {
	slog.Info("Generating Trace Events")
	nonHpcLabelSelector := "app=" + v.NonHpcAppName
	ebpfLabelSelector := "name=" + v.EbpfXdpDeamonSetName

	// TRACE
	output, err := kubernetes.ExecCommandInWinPod(
		v.KubeConfigFilePath,
		"C:\\event-writer-helper.bat EventWriter-SetFilter -event 4 -srcIP "+TestExternalIPAddress,
		v.EbpfXdpDeamonSetNamespace,
		ebpfLabelSelector,
		true,
	)
	if err != nil {
		return fmt.Errorf("executing EventWriter command: %w", err)
	}

	if strings.Contains(output, "failed") || strings.Contains(output, "error") || strings.Contains(output, "exiting") {
		return ErrSetFilterEventWriter
	}

	numcurls := 10
	for numcurls > 0 {
		_, err = kubernetes.ExecCommandInWinPod(
			v.KubeConfigFilePath,
			"C:\\event-writer-helper.bat EventWriter-Curl "+TestExternalIPAddress,
			v.NonHpcAppNamespace,
			nonHpcLabelSelector,
			false,
		)
		if err != nil {
			return fmt.Errorf("executing EventWriter command: %w", err)
		}
		numcurls--
	}

	return nil
}

func (v *ValidateWinBpfMetric) generateDropEvents() error {
	slog.Info("Generating Drop Events")
	nonHpcLabelSelector := "app=" + v.NonHpcAppName
	ebpfLabelSelector := "name=" + v.EbpfXdpDeamonSetName

	output, err := kubernetes.ExecCommandInWinPod(
		v.KubeConfigFilePath,
		"C:\\event-writer-helper.bat EventWriter-SetFilter -event 1 -srcIP "+TestExternalIPAddress,
		v.EbpfXdpDeamonSetNamespace,
		ebpfLabelSelector,
		true,
	)
	if err != nil {
		return fmt.Errorf("executing EventWriter command: %w", err)
	}

	if strings.Contains(output, "failed") || strings.Contains(output, "error") || strings.Contains(output, "exiting") {
		return ErrStartEventWriter
	}

	numcurls := 10
	for numcurls > 0 {
		_, err = kubernetes.ExecCommandInWinPod(
			v.KubeConfigFilePath,
			"C:\\event-writer-helper.bat EventWriter-Curl "+TestExternalIPAddress,
			v.NonHpcAppNamespace,
			nonHpcLabelSelector,
			false,
		)
		if err != nil {
			return fmt.Errorf("executing EventWriter command: %w", err)
		}
		numcurls--
	}

	return nil
}

func (v *ValidateWinBpfMetric) generatePktmonDropEvents() error {
	slog.Info("Generating Drop Events")
	nonHpcLabelSelector := "app=" + v.NonHpcAppName
	ebpfLabelSelector := "name=" + v.EbpfXdpDeamonSetName

	output, err := kubernetes.ExecCommandInWinPod(
		v.KubeConfigFilePath,
		"C:\\event-writer-helper.bat EventWriter-SetFilter -event 100 -srcIP "+TestExternalIPAddress,
		v.EbpfXdpDeamonSetNamespace,
		ebpfLabelSelector,
		true,
	)
	if err != nil {
		return fmt.Errorf("executing EventWriter command: %w", err)
	}

	if strings.Contains(output, "failed") || strings.Contains(output, "error") || strings.Contains(output, "exiting") {
		return ErrStartEventWriter
	}

	numcurls := 10
	for numcurls > 0 {
		_, err = kubernetes.ExecCommandInWinPod(
			v.KubeConfigFilePath,
			"C:\\event-writer-helper.bat EventWriter-Curl "+TestExternalIPAddress,
			v.NonHpcAppNamespace,
			nonHpcLabelSelector,
			false,
		)
		if err != nil {
			return fmt.Errorf("executing EventWriter command: %w", err)
		}
		numcurls--
	}

	return nil
}

func (v *ValidateWinBpfMetric) verifyBasicMetrics(promOutput string) error {
	var fwdBytes float64
	var drpBytes float64
	var windowsDrpBytes float64
	var fwdCount float64
	var drpCount float64
	var windowsDrpCount float64

	fwdLabels := map[string]string{
		"direction": "ingress",
	}

	drpLabels := map[string]string{
		"direction": "ingress",
		"reason":    "130, 0",
	}

	windowsDrpLabels := map[string]string{
		"direction": "ingress",
		"reason":    "DropReason_PacketMonitor, Drop_FL_InterfaceNotReady",
	}

	if promOutput == "" {
		slog.Info("No Prometheus metrics found, skipping validation")
	} else {
		// Forward event
		err := prom.CheckMetricFromBuffer([]byte(promOutput), "networkobservability_forward_bytes", fwdLabels)
		if err != nil {
			return fmt.Errorf("failed to verify prometheus metrics: %w", err)
		}

		fwdBytes, err = prom.GetMetricGuageValueFromBuffer([]byte(promOutput), "networkobservability_forward_bytes", fwdLabels)
		if err != nil {
			return fmt.Errorf("failed to get forward bytes metric: %w", err)
		}
		slog.Info("networkobservability_forward_bytes value", "value", fwdBytes, "labels", fwdLabels)
		if fwdBytes == 0 {
			return ErrForwardBytesZero
		}

		fwdCount, err = prom.GetMetricGuageValueFromBuffer([]byte(promOutput), "networkobservability_forward_count", fwdLabels)
		if err != nil {
			return fmt.Errorf("failed to get forward count metric: %w", err)
		}
		slog.Info("networkobservability_forward_count value", "value", fwdCount, "labels", fwdLabels)
		if fwdCount == 0 {
			return ErrForwardCountZero
		}

		// Drop event
		drpBytes, err = prom.GetMetricGuageValueFromBuffer([]byte(promOutput), "networkobservability_drop_bytes", drpLabels)
		if err != nil {
			return fmt.Errorf("failed to get drop bytes metric: %w", err)
		}
		slog.Info("networkobservability_drop_bytes value", "value", drpBytes, "labels", drpLabels)
		if drpBytes == 0 {
			return ErrDropBytesZero
		}

		drpCount, err = prom.GetMetricGuageValueFromBuffer([]byte(promOutput), "networkobservability_drop_count", drpLabels)
		if err != nil {
			return fmt.Errorf("failed to get drop count metric: %w", err)
		}
		slog.Info("networkobservability_drop_count value", "value", drpCount, "labels", drpLabels)
		if drpCount == 0 {
			return ErrDropCountZero
		}

		// Windows drop event
		windowsDrpBytes, err = prom.GetMetricGuageValueFromBuffer([]byte(promOutput), "networkobservability_drop_bytes", windowsDrpLabels)
		if err != nil {
			return fmt.Errorf("failed to get windows drop bytes metric: %w", err)
		}
		slog.Info("networkobservability_drop_bytes (windows) value", "value", windowsDrpBytes, "labels", windowsDrpLabels)
		if windowsDrpBytes == 0 {
			return ErrWindowsDropBytesZero
		}

		windowsDrpCount, err = prom.GetMetricGuageValueFromBuffer([]byte(promOutput), "networkobservability_drop_count", windowsDrpLabels)
		if err != nil {
			return fmt.Errorf("failed to get windows drop count metric: %w", err)
		}
		slog.Info("networkobservability_drop_count (windows) value", "value", windowsDrpCount, "labels", windowsDrpLabels)
		if windowsDrpCount == 0 {
			return ErrWindowsDropCountZero
		}
	}

	return nil
}

func (v *ValidateWinBpfMetric) verifyAdvancedMetrics(nonHpcIPAddr, promOutput string) error {
	// Advanced Metrics
	advFwdCountLabels := map[string]string{
		"direction":     "egress",
		"ip":            "23.192.228.84",
		"namespace":     "",
		"podname":       "",
		"workload_kind": "unknown",
		"workload_name": "unknown",
	}
	err := prom.CheckMetricFromBuffer([]byte(promOutput), "networkobservability_adv_forward_count", advFwdCountLabels)
	if err != nil {
		return fmt.Errorf("failed to find networkobservability_adv_forward_count: %w", err)
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

	advDropByteLabels := map[string]string{
		"direction":     "egress",
		"ip":            "23.192.228.84",
		"namespace":     "",
		"podname":       "",
		"reason":        "Reason_LbNoBackend",
		"workload_kind": "unknown",
		"workload_name": "unknown",
	}
	err = prom.CheckMetricFromBuffer([]byte(promOutput), "networkobservability_adv_drop_bytes", advDropByteLabels)
	if err != nil {
		return fmt.Errorf("failed to find networkobservability_adv_drop_bytes: %w", err)
	}

	advDropCountLabels := map[string]string{
		"direction":     "egress",
		"ip":            "23.192.228.84",
		"namespace":     "",
		"podname":       "",
		"reason":        "Reason_LbNoBackend",
		"workload_kind": "unknown",
		"workload_name": "unknown",
	}
	err = prom.CheckMetricFromBuffer([]byte(promOutput), "networkobservability_adv_drop_count", advDropCountLabels)
	if err != nil {
		return fmt.Errorf("failed to find networkobservability_adv_drop_count: %w", err)
	}

	advPktmonDropCountLabels := map[string]string{
		"direction":     "egress",
		"ip":            "23.192.228.84",
		"namespace":     "",
		"podname":       "",
		"reason":        "Drop_Busy",
		"workload_kind": "unknown",
		"workload_name": "unknown",
	}

	err = prom.CheckMetricFromBuffer([]byte(promOutput), "networkobservability_adv_drop_count", advPktmonDropCountLabels)
	if err != nil {
		return fmt.Errorf("failed to find networkobservability_adv_drop_count: %w", err)
	}

	advFwdCountLabels = map[string]string{
		"direction":     "ingress",
		"ip":            nonHpcIPAddr,
		"namespace":     v.NonHpcAppNamespace,
		"podname":       v.NonHpcPodName,
		"workload_kind": "unknown",
		"workload_name": "unknown",
	}
	err = prom.CheckMetricFromBuffer([]byte(promOutput), "networkobservability_adv_forward_count", advFwdCountLabels)
	if err != nil {
		return fmt.Errorf("failed to find networkobservability_adv_forward_count: %w", err)
	}

	for _, flag := range tcpFlags {
		tcpFlagLabels := map[string]string{
			"flag":          flag,
			"ip":            nonHpcIPAddr,
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

	advDropByteLabels = map[string]string{
		"direction":     "ingress",
		"ip":            nonHpcIPAddr,
		"namespace":     v.NonHpcAppNamespace,
		"podname":       v.NonHpcPodName,
		"reason":        "Reason_LbNoBackend",
		"workload_kind": "unknown",
		"workload_name": "unknown",
	}
	err = prom.CheckMetricFromBuffer([]byte(promOutput), "networkobservability_adv_drop_bytes", advDropByteLabels)
	if err != nil {
		return fmt.Errorf("failed to find networkobservability_adv_drop_bytes with ingress label: %w", err)
	}

	advDropCountLabels = map[string]string{
		"direction":     "ingress",
		"ip":            nonHpcIPAddr,
		"namespace":     v.NonHpcAppNamespace,
		"podname":       v.NonHpcPodName,
		"reason":        "Reason_LbNoBackend",
		"workload_kind": "unknown",
		"workload_name": "unknown",
	}
	err = prom.CheckMetricFromBuffer([]byte(promOutput), "networkobservability_adv_drop_count", advDropCountLabels)
	if err != nil {
		return fmt.Errorf("failed to find networkobservability_adv_drop_count with ingress label: %w", err)
	}
	return nil
}

func (v *ValidateWinBpfMetric) Run() error {
	nonHpcLabelSelector := "app=" + v.NonHpcAppName
	slog.Info("Waiting for Non HPC Pod to come up")
	// Wait for the non HPC pod to be ready. Maximum wait time is 15 minutes in case the Pods are very slow to come up.
	if err := kubernetes.WaitForPodReadyWithTimeOut(context.TODO(), v.KubeConfigFilePath, v.NonHpcAppNamespace, nonHpcLabelSelector, 15*time.Minute); err != nil {
		slog.Warn("waiting for Non HPC Pod ready timed out", "error", err)
	}
	slog.Info("Non HPC Pod is ready")

	nonHpcIPAddr, err := v.getNonHpcPodIPAddress()
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

	// generate pktmon drop events
	err = v.generatePktmonDropEvents()
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

	err = v.verifyAdvancedMetrics(nonHpcIPAddr, promOutput)
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
