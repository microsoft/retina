package windows

import (
	"context"
	"fmt"
	"strings"
	"time"

	kubernetes "github.com/microsoft/retina/test/e2e/framework/kubernetes"
	prom "github.com/microsoft/retina/test/e2e/framework/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
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

	for i := 0; i < attempts; i++ {
		promOutput, err = kubernetes.ExecCommandInWinPod(
			v.KubeConfigFilePath,
			"C:\\event-writer-helper.bat EventWriter-GetRetinaPromMetrics",
			v.RetinaDaemonSetNamespace,
			retinaLabelSelector,
		)

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

func waitForPodReadyWithClientGo(ctx context.Context, clientset *k8s.Clientset, namespace, labelSelector string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
		if err == nil && len(pods.Items) > 0 {
			for _, cond := range pods.Items[0].Status.Conditions {
				if cond.Type == "Ready" && cond.Status == "True" {
					return nil
				}
			}
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("timeout waiting for pod to become ready")
}

func (v *ValidateWinBpfMetric) Run() error {
	fmt.Println("Entering ValidateWinBpfMetric::Run method")
	ebpfLabelSelector := fmt.Sprintf("name=%s", v.EbpfXdpDeamonSetName)
	promOutput, err := v.GetPromMetrics()
	if err != nil {
		return err
	}

	/*fwd_labels := map[string]string{
		"direction": "ingress",
	}
	drp_labels := map[string]string{
		"direction": "ingress",
		"reason":    "130, 0",
	}*/

	fmt.Println("Pre test - Prometheus metrics output:")
	fmt.Println(promOutput)

	/*var preTestFwdBytes float64 = 0
	var preTestDrpBytes float64 = 0
	var preTestFwdCount float64 = 0
	var preTestDrpCount float64 = 0

	if promOutput == "" {
		fmt.Println("Pre test - no prometheus metrics found")
	} else {
		err = prom.CheckMetricFromBuffer([]byte(promOutput), "networkobservability_forward_bytes", fwd_labels)
		if err != nil {
			return fmt.Errorf("failed to verify prometheus metrics: %w", err)
		}

		preTestFwdBytes, err = prom.GetMetricGuageValueFromBuffer([]byte(promOutput), "networkobservability_forward_bytes", fwd_labels)
		if err != nil && strings.Contains(err.Error(), "failed to parse prometheus metrics") {
			return err
		}
		fmt.Printf("Pre test - networkobservability_forward_bytes value %f, labels: %v\n", preTestFwdBytes, fwd_labels)

		preTestFwdCount, err = prom.GetMetricGuageValueFromBuffer([]byte(promOutput), "networkobservability_forward_count", fwd_labels)
		if err != nil && strings.Contains(err.Error(), "failed to parse prometheus metrics") {
			return err
		}
		fmt.Printf("Pre test - networkobservability_forward_count value %f, labels: %v\n", preTestFwdCount, fwd_labels)

		preTestDrpBytes, err = prom.GetMetricGuageValueFromBuffer([]byte(promOutput), "networkobservability_drop_bytes", drp_labels)
		if err != nil && strings.Contains(err.Error(), "failed to parse prometheus metrics") {
			return err
		}
		fmt.Printf("Pre test - networkobservability_drop_bytes value %f, labels: %v\n", preTestDrpBytes, drp_labels)

		preTestDrpCount, err = prom.GetMetricGuageValueFromBuffer([]byte(promOutput), "networkobservability_drop_count", drp_labels)
		if err != nil && strings.Contains(err.Error(), "failed to parse prometheus metrics") {
			return err
		}
		fmt.Printf("Pre test - networkobservability_drop_count value %f, labels: %v\n", preTestDrpCount, drp_labels)
	}*/

	fmt.Println("Waiting for Non HPC Pod to come up")
	nonHpcLabelSelector := fmt.Sprintf("app=%s", v.NonHpcAppName)

	config, _ := clientcmd.BuildConfigFromFlags("", v.KubeConfigFilePath)
	clientset, _ := k8s.NewForConfig(config)
	waitForPodReadyWithClientGo(context.TODO(), clientset, v.NonHpcAppNamespace, nonHpcLabelSelector, 10*time.Minute)
	fmt.Println("Non HPC Pod is ready")

	fmt.Println("Executing  EventWriter-GetPodIpAddress")
	nonHpcIpAddr, err := kubernetes.ExecCommandInWinPod(
		v.KubeConfigFilePath,
		"C:\\event-writer-helper.bat EventWriter-GetPodIpAddress",
		v.NonHpcAppNamespace,
		nonHpcLabelSelector)
	if err != nil {
		return err
	}
	nonHpcIpAddr = strings.TrimSpace(nonHpcIpAddr)

	if strings.Contains(nonHpcIpAddr, "failed") || strings.Contains(nonHpcIpAddr, "error") {
		return fmt.Errorf("failed to get nonHpcIpAddr")
	}
	fmt.Println("Non HPC IP Addr: ", nonHpcIpAddr)

	nonHpcIfIndex, err := kubernetes.ExecCommandInWinPod(
		v.KubeConfigFilePath,
		"C:\\event-writer-helper.bat EventWriter-GetPodIfIndex",
		v.NonHpcAppNamespace,
		nonHpcLabelSelector)
	if err != nil {
		return err
	}
	if strings.Contains(nonHpcIfIndex, "failed") || strings.Contains(nonHpcIfIndex, "error") {
		return fmt.Errorf("failed to get nonHpcIfIndex")
	}
	fmt.Println("Non HPC Interface Index: ", nonHpcIfIndex)

	//Attach to the non HPC pod
	output, err := kubernetes.ExecCommandInWinPod(
		v.KubeConfigFilePath,
		fmt.Sprintf("C:\\event-writer-helper.bat EventWriter-Attach %s", nonHpcIfIndex),
		v.EbpfXdpDeamonSetNamespace,
		ebpfLabelSelector)
	if err != nil {
		return err
	}
	fmt.Println(output)
	if strings.Contains(output, "failed") || strings.Contains(output, "error") || strings.Contains(output, "exiting") {
		return fmt.Errorf("failed to attach to non HPC pod interface %s", output)
	}

	//TRACE
	fmt.Printf("Produce Trace Events\n")
	//Example.com - 23.192.228.84
	output, err = kubernetes.ExecCommandInWinPod(
		v.KubeConfigFilePath,
		"C:\\event-writer-helper.bat EventWriter-SetFilter -event 4 -srcIP 23.192.228.84",
		v.EbpfXdpDeamonSetNamespace,
		ebpfLabelSelector)
	if err != nil {
		return err
	}

	fmt.Println(output)
	if strings.Contains(output, "failed") || strings.Contains(output, "error") || strings.Contains(output, "exiting") {
		return fmt.Errorf("failed to set filter for event writer")
	}

	numcurls := 10
	for numcurls > 0 {
		_, err = kubernetes.ExecCommandInWinPod(
			v.KubeConfigFilePath,
			"C:\\event-writer-helper.bat EventWriter-Curl 23.192.228.84",
			v.NonHpcAppNamespace,
			nonHpcLabelSelector)
		if err != nil {
			return err
		}
		numcurls--
	}

	//DROP
	time.Sleep(20 * time.Second)
	fmt.Printf("Produce Drop Events\n")
	output, err = kubernetes.ExecCommandInWinPod(
		v.KubeConfigFilePath,
		"C:\\event-writer-helper.bat EventWriter-SetFilter -event 1 -srcIP 23.192.228.84",
		v.EbpfXdpDeamonSetNamespace,
		ebpfLabelSelector)
	if err != nil {
		return err
	}

	if strings.Contains(output, "failed") || strings.Contains(output, "error") || strings.Contains(output, "exiting") {
		return fmt.Errorf("failed to start event writer")
	}

	numcurls = 10
	for numcurls > 0 {
		_, err = kubernetes.ExecCommandInWinPod(
			v.KubeConfigFilePath,
			"C:\\event-writer-helper.bat EventWriter-Curl 23.192.228.84",
			v.NonHpcAppNamespace,
			nonHpcLabelSelector)
		if err != nil {
			return err
		}
		numcurls--
	}

	fmt.Println("Waiting for basic metrics to be updated as part of next polling cycle")
	time.Sleep(60 * time.Second)
	promOutput, err = v.GetPromMetrics()
	if err != nil {
		return err
	}

	//TBR
	fmt.Println(promOutput)

	// Advanced Metrics
	adv_fwd_count_labels := map[string]string{
		"direction":     "egress",
		"ip":            "23.192.228.84",
		"namespace":     "",
		"podname":       "",
		"workload_kind": "unknown",
		"workload_name": "unknown",
	}
	err = prom.CheckMetricFromBuffer([]byte(promOutput), "networkobservability_adv_forward_count", adv_fwd_count_labels)
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
		fmt.Printf("Found TCP flag metric for %s\n", flag)
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
		fmt.Printf("Found TCP flag metric for %s\n", flag)
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

func (v *ValidateWinBpfMetric) Prevalidate() error {
	return nil
}

func (v *ValidateWinBpfMetric) Stop() error {
	return nil
}
