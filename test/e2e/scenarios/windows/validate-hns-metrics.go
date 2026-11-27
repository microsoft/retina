package windows

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/microsoft/retina/test/e2e/framework/constants"
	k8s "github.com/microsoft/retina/test/e2e/framework/kubernetes"
	prom "github.com/microsoft/retina/test/e2e/framework/prometheus"
	"github.com/microsoft/retina/test/retry"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetes "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	defaultRetryDelay        = 5 * time.Second
	defaultRetryAttempts     = 5
	defaultHTTPClientTimeout = 2 * time.Second
)

var (
	ErrorNoWindowsPod = errors.New("no windows retina pod found")
	ErrNoMetricFound  = fmt.Errorf("no metric found")

	hnsMetricName  = "networkobservability_windows_hns_stats"
	defaultRetrier = retry.Retrier{Attempts: defaultRetryAttempts, Delay: defaultRetryDelay}
)

type ValidateHNSMetric struct {
	Year                     string
	KubeConfigFilePath       string
	RetinaDaemonSetNamespace string
	RetinaDaemonSetName      string
	NonHpcPodName            string
	NonHpcAppName            string
}

func (v *ValidateHNSMetric) Run() error {
	err := k8s.WaitForPodReadyWithTimeOut(context.Background(), v.KubeConfigFilePath, "default", "app="+v.NonHpcAppName, 15*time.Minute)
	if err != nil {
		return fmt.Errorf("waiting for the Windows HNS workload: %w", err)
	}
	config, err := clientcmd.BuildConfigFromFlags("", v.KubeConfigFilePath)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("error creating Kubernetes client: %w", err)
	}

	targetNodeName, err := validateWorkloadNode(context.TODO(), clientset, "default", v.NonHpcPodName, v.Year)
	if err != nil {
		return err
	}

	pods, err := clientset.CoreV1().Pods(v.RetinaDaemonSetNamespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: retinaLabelSelector,
	})
	if err != nil {
		return fmt.Errorf("listing Windows Retina pods: %w", err)
	}

	var windowsRetinaPod *v1.Pod
	for pod := range pods.Items {
		if pods.Items[pod].Spec.NodeSelector["kubernetes.io/os"] == "windows" && pods.Items[pod].Spec.NodeName == targetNodeName {
			windowsRetinaPod = &pods.Items[pod]
			break
		}
	}
	if windowsRetinaPod == nil {
		return fmt.Errorf("%w on node %s", ErrorNoWindowsPod, targetNodeName)
	}

	labels := map[string]string{
		"direction": "win_packets_sent_count",
	}

	log.Printf("checking for metric %s with labels %+v\n", hnsMetricName, labels)

	// wrap this in a retrier because windows is slow
	var output []byte
	err = defaultRetrier.Do(context.TODO(), func() error {
		output, err = k8s.ExecPod(context.TODO(), clientset, config, windowsRetinaPod.Namespace, windowsRetinaPod.Name, fmt.Sprintf("curl -s http://localhost:%s/metrics", constants.RetinaMetricsPort))
		if err != nil {
			return fmt.Errorf("error executing command in windows retina pod: %w", err)
		}
		if len(output) == 0 {
			return ErrNoMetricFound
		}

		if err != nil {
			return fmt.Errorf("failed to get metrics from windows retina pod: %w", err)
		}

		err = prom.CheckMetricFromBuffer(output, hnsMetricName, labels)
		if err != nil {
			return fmt.Errorf("failed to verify prometheus metrics: %w", err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("validating Windows HNS metrics: %w", err)
	}
	log.Printf("found metric matching %+v: with labels %+v\n", hnsMetricName, labels)
	return nil
}

func (v *ValidateHNSMetric) Prevalidate() error {
	return nil
}

func (v *ValidateHNSMetric) Stop() error {
	return nil
}
