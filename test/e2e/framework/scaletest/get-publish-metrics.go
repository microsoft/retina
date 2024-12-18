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
	"github.com/microsoft/retina/test/retry"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	v1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"
)

const (
	defaultRetryAttempts = 10
	defaultRetryDelay    = 500 * time.Millisecond
)

type GetAndPublishMetrics struct {
	KubeConfigFilePath          string
	AdditionalTelemetryProperty map[string]string
	Labels                      map[string]string
	outputFilePath              string
	stop                        chan struct{}
	wg                          sync.WaitGroup
	telemetryClient             *telemetry.TelemetryClient
	appInsightsKey              string
	k8sClient                   *kubernetes.Clientset
	metricsClient               *metrics.Clientset
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

	config, err := clientcmd.BuildConfigFromFlags("", g.KubeConfigFilePath)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %w", err)
	}

	k8sClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("error creating Kubernetes client: %w", err)
	}
	g.k8sClient = k8sClient

	metricsClient, err := metrics.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("error creating metrics client: %w", err)
	}
	g.metricsClient = metricsClient

	g.stop = make(chan struct{})
	g.wg.Add(1)

	go func() {

		t := time.NewTicker(5 * time.Minute)

		// First execution
		err := g.getAndPublishMetrics()
		if err != nil {
			log.Fatalf("error getting and publishing number of restarts: %v", err)
			return
		}

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

	if os.Getenv(common.OutputFilePathEnv) == "" {
		log.Println("Output file path not provided. Metrics will not be written to file")
		return nil
	}
	g.outputFilePath = os.Getenv(common.OutputFilePathEnv)

	log.Println("Output file path provided: ", g.outputFilePath)
	return nil
}

func (g *GetAndPublishMetrics) getAndPublishMetrics() error {

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeoutSeconds*time.Second)
	defer cancel()

	labelSelector := labels.Set(g.Labels).String()

	agentsMetrics, err := g.getPodsMetrics(ctx, labelSelector)
	if err != nil {
		log.Println("Error getting agents' metrics, will try again later:", err)
		return nil
	}

	operatorMetrics, err := g.getPodsMetrics(ctx, "app=retina-operator")
	if err != nil {
		log.Println("Error getting operator's metrics, will try again later:", err)
		return nil
	}

	metrics := append(agentsMetrics, operatorMetrics...)

	// Publish metrics
	if g.telemetryClient != nil {
		log.Println("Publishing metrics to AppInsights")
		for _, metric := range metrics {
			g.telemetryClient.TrackEvent("scale-test", metric)

		}
	}

	// Write metrics to file
	if g.outputFilePath != "" {
		log.Println("Writing metrics to file ", g.outputFilePath)
		file, err := os.OpenFile(g.outputFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
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

func (g *GetAndPublishMetrics) getPodsMetrics(ctx context.Context, labelSelector string) ([]metric, error) {

	var pods *v1.PodList

	retrier := retry.Retrier{Attempts: defaultRetryAttempts, Delay: defaultRetryDelay}

	err := retrier.Do(ctx, func() error {
		var err error
		pods, err = g.k8sClient.CoreV1().Pods(common.KubeSystemNamespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
		return err
	})
	if err != nil {
		return nil, errors.Wrap(err, "error getting pods")
	}

	var nodeMetricsList *v1beta1.NodeMetricsList
	err = retrier.Do(ctx, func() error {
		var err error
		nodeMetricsList, err = g.metricsClient.MetricsV1beta1().NodeMetricses().List(ctx, metav1.ListOptions{})
		return err
	})
	if err != nil {
		log.Println("Error getting node metrics:", err)
	}

	var podMetricsList *v1beta1.PodMetricsList
	err = retrier.Do(ctx, func() error {
		var err error
		podMetricsList, err = g.metricsClient.MetricsV1beta1().PodMetricses(common.KubeSystemNamespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
		return err
	})
	if err != nil {
		log.Println("Error getting pod metrics:", err)
	}

	var allPodsHealth []metric

	timestamp := time.Now().UTC().Format(time.RFC3339)

	// List -> map for lookup
	podMetrics := make(map[string]*v1beta1.PodMetrics)
	for _, pm := range podMetricsList.Items {
		podMetrics[pm.Name] = pm.DeepCopy()
	}

	// List -> map for lookup
	nodeMetrics := make(map[string]*v1beta1.NodeMetrics)
	for _, nm := range nodeMetricsList.Items {
		nodeMetrics[nm.Name] = nm.DeepCopy()
	}

	for _, pod := range pods.Items {
		var podHealth metric = make(map[string]string)

		podMem := resource.MustParse("0")
		podCpu := resource.MustParse("0")
		if podMetrics[pod.Name] != nil {
			for _, cm := range podMetrics[pod.Name].Containers {
				podMem.Add(cm.Usage["memory"])
				podCpu.Add(cm.Usage["cpu"])
			}
		}

		nodeMem := resource.MustParse("0")
		nodeCpu := resource.MustParse("0")
		if nodeMetrics[pod.Spec.NodeName] != nil {
			nodeMem = nodeMetrics[pod.Spec.NodeName].Usage["memory"]
			nodeCpu = nodeMetrics[pod.Spec.NodeName].Usage["cpu"]
		}

		restarts := 0

		for _, containerStatus := range pod.Status.ContainerStatuses {
			restarts = restarts + int(containerStatus.RestartCount)
		}

		podHealth["timestamp"] = timestamp
		podHealth["pod"] = pod.Name
		podHealth["podStatus"] = string(pod.Status.Phase)
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
