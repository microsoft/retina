package scaletest

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/microsoft/retina/pkg/telemetry"
	"github.com/microsoft/retina/test/e2e/common"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"
)

type GetAndPublishMetrics struct {
	KubeConfigFilePath          string
	AdditionalTelemetryProperty map[string]string
	Labels                      map[string]string
	OutputFilePath              string
	stop                        chan struct{}
	wg                          sync.WaitGroup
	telemetryClient             *telemetry.TelemetryClient
	appInsightsKey              string
}

func (g *GetAndPublishMetrics) Run() error {
	if g.appInsightsKey != "" {
		telemetry.InitAppInsights(g.appInsightsKey, g.AdditionalTelemetryProperty["retinaVersion"])

		telemetryClient, err := telemetry.NewAppInsightsTelemetryClient("retina-scale-test", g.AdditionalTelemetryProperty)
		if err != nil {
			return errors.Wrap(err, "error creating telemetry client")
		}

		g.telemetryClient = telemetryClient
	}

	g.stop = make(chan struct{})
	g.wg.Add(1)

	go func() {
		t := time.NewTicker(5 * time.Minute)

		for {
			select {

			case <-t.C:
				err := g.getAndPublishMetrics()
				if err != nil {
					log.Fatalf("error getting and publishing number of restarts: %v", err)
					return
				}

			case <-g.stop:
				g.wg.Done()
				return

			}
		}
	}()

	return nil
}

func (g *GetAndPublishMetrics) Stop() error {
	telemetry.ShutdownAppInsights()
	close(g.stop)
	g.wg.Wait()
	return nil
}

func (g *GetAndPublishMetrics) Prevalidate() error {
	if os.Getenv(common.AzureAppInsightsKeyEnv) == "" {
		log.Println("env ", common.AzureAppInsightsKeyEnv, " not provided")
	}
	g.appInsightsKey = os.Getenv(common.AzureAppInsightsKeyEnv)

	if _, ok := g.AdditionalTelemetryProperty["retinaVersion"]; !ok {
		return fmt.Errorf("retinaVersion is required in AdditionalTelemetryProperty")
	}
	return nil
}

func (g *GetAndPublishMetrics) getAndPublishMetrics() error {
	config, err := clientcmd.BuildConfigFromFlags("", g.KubeConfigFilePath)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("error creating Kubernetes client: %w", err)
	}

	mc, err := metrics.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("error creating metrics client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeoutSeconds*time.Second)
	defer cancel()

	metrics, err := g.getMetrics(ctx, clientset, mc)
	if err != nil {
		return fmt.Errorf("error getting metrics: %w", err)
	}

	// Publish metrics
	if g.telemetryClient != nil {
		log.Println("Publishing metrics to AppInsights")
		for _, metric := range metrics {
			g.telemetryClient.TrackEvent("scale-test", metric)
		}
	}

	// Write metrics to file
	if g.OutputFilePath != "" {
		log.Println("Writing metrics to file ", g.OutputFilePath)
		file, err := os.OpenFile(g.OutputFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return fmt.Errorf("error writing to csv file: %w", err)
		}
		defer file.Close()

		for _, m := range metrics {
			b, err := json.Marshal(m)
			if err != nil {
				return fmt.Errorf("error marshalling metric: %w", err)
			}
			file.Write(b)
			file.WriteString("\n")
		}

	}

	return nil
}

type metric map[string]string

func (g *GetAndPublishMetrics) getMetrics(ctx context.Context, k8sClient *kubernetes.Clientset, metricsClient *metrics.Clientset) ([]metric, error) {
	labelSelector := labels.Set(g.Labels).String()

	pods, err := k8sClient.CoreV1().Pods(common.KubeSystemNamespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return nil, errors.Wrap(err, "error getting nodes")
	}

	nodesMetricsInt := metricsClient.MetricsV1beta1().NodeMetricses()
	podMetricsInt := metricsClient.MetricsV1beta1().PodMetricses(common.KubeSystemNamespace)

	var allPodsHealth []metric

	timestamp := time.Now().UTC().Format(time.RFC3339)

	for _, pod := range pods.Items {
		var podHealth metric = make(map[string]string)

		podMetrics, err := podMetricsInt.Get(ctx, pod.Name, metav1.GetOptions{})
		if err != nil {
			return nil, errors.Wrap(err, "error getting pod metrics")
		}

		podMem := resource.MustParse("0")
		podCpu := resource.MustParse("0")
		for _, cm := range podMetrics.Containers {
			podMem.Add(cm.Usage["memory"])
			podCpu.Add(cm.Usage["cpu"])
		}

		nodeMetrics, err := nodesMetricsInt.Get(ctx, pod.Spec.NodeName, metav1.GetOptions{})
		if err != nil {
			return nil, errors.Wrap(err, "error getting node metrics")
		}

		nodeMem := nodeMetrics.Usage["memory"]
		nodeCpu := nodeMetrics.Usage["cpu"]

		restarts := 0

		for _, containerStatus := range pod.Status.ContainerStatuses {
			restarts = restarts + int(containerStatus.RestartCount)
		}

		podHealth["timestamp"] = timestamp
		podHealth["pod"] = pod.Name
		podHealth["podCpuInMilliCore"] = fmt.Sprintf("%d", podCpu.MilliValue())
		podHealth["podMemoryInMB"] = fmt.Sprintf("%d", podMem.Value()/(1048576))
		podHealth["podRestarts"] = fmt.Sprintf("%d", restarts)
		podHealth["node"] = pod.Spec.NodeName
		podHealth["nodeCpuInMilliCore"] = fmt.Sprintf("%d", nodeCpu.MilliValue())
		podHealth["nodeMemoryInMB"] = fmt.Sprintf("%d", nodeMem.Value()/(1048576))

		allPodsHealth = append(allPodsHealth, podHealth)

	}

	return allPodsHealth, nil
}
