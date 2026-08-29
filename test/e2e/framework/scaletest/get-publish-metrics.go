package scaletest

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/microsoft/retina/pkg/telemetry"
	"github.com/microsoft/retina/test/e2e/common"
	"github.com/microsoft/retina/test/retry"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
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
	defaultRetryDelay    = 3 * time.Second
	defaultInterval      = 2 * time.Minute
)

type GetAndPublishMetrics struct {
	KubeConfigFilePath          string
	AdditionalTelemetryProperty map[string]string
	Labels                      map[string]string
	outputFilePath              string
	stop                        chan struct{}
	errs                        *errgroup.Group
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
	g.errs = new(errgroup.Group)

	g.errs.Go(func() error {

		t := time.NewTicker(defaultInterval)
		defer t.Stop()

		// First execution
		err := g.getAndPublishMetrics()
		if err != nil {
			return fmt.Errorf("failed to get and publish test metrics: %w", err)
		}

		for {
			select {

			case <-t.C:
				err := g.getAndPublishMetrics()
				if err != nil {
					return fmt.Errorf("failed to get and publish test metrics: %w", err)
				}

			case <-g.stop:
				return nil

			}
		}

	})

	return nil
}

func (g *GetAndPublishMetrics) Stop() error {
	telemetry.ShutdownAppInsights()
	close(g.stop)
	if err := g.errs.Wait(); err != nil {
		return err //nolint:wrapcheck // already wrapped in goroutine
	}

	return nil
}

func (g *GetAndPublishMetrics) Prevalidate() error {
	if g.appInsightsKey == "" {
		log.Println("env ", common.AzureAppInsightsKeyEnv, " not provided")
	}

	if _, ok := g.AdditionalTelemetryProperty["retinaVersion"]; !ok {
		return fmt.Errorf("retinaVersion is required in AdditionalTelemetryProperty")
	}

	if g.outputFilePath == "" {
		log.Println("Output file path not provided. Metrics will not be written to file")
		return nil
	}

	log.Println("Output file path provided: ", g.outputFilePath)
	return nil
}

func (g *GetAndPublishMetrics) getAndPublishMetrics() error {

	ctx := context.TODO()

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

	allMetrics := []metric{}
	allMetrics = append(allMetrics, agentsMetrics...)
	allMetrics = append(allMetrics, operatorMetrics...)

	// Publish metrics
	if g.telemetryClient != nil {
		log.Println("Publishing metrics to AppInsights")
		for _, metric := range allMetrics {
			g.telemetryClient.TrackEvent("scale-test", metric)

		}
	}

	// Write metrics to file
	if g.outputFilePath != "" {
		log.Println("Writing metrics to file ", g.outputFilePath)

		file, err := os.OpenFile(g.outputFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return fmt.Errorf("error writing to csv file: %w", err)
		}
		defer file.Close()

		for _, m := range allMetrics {
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
		if err != nil {
			return fmt.Errorf("error listing pods: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, errors.Wrap(err, "error getting pods")
	}

	var nodeMetricsList *v1beta1.NodeMetricsList
	err = retrier.Do(ctx, func() error {
		nodeMetricsList, err = g.metricsClient.MetricsV1beta1().NodeMetricses().List(ctx, metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("error listing node metrics: %w", err)
		}
		return nil
	})
	if err != nil {
		log.Println("Error getting node metrics:", err)
	}

	var podMetricsList *v1beta1.PodMetricsList
	err = retrier.Do(ctx, func() error {
		podMetricsList, err = g.metricsClient.MetricsV1beta1().PodMetricses(common.KubeSystemNamespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
		if err != nil {
			return fmt.Errorf("error listing pod metrics: %w", err)
		}
		return nil
	})
	if err != nil {
		log.Println("Error getting pod metrics:", err)
	}

	var allPodsHealth []metric

	timestamp := time.Now().UTC().Format(time.RFC3339)

	// List -> map for lookup
	podMetrics := make(map[string]*v1beta1.PodMetrics)
	for i := range podMetricsList.Items {
		podMetrics[podMetricsList.Items[i].Name] = podMetricsList.Items[i].DeepCopy()
	}

	// List -> map for lookup
	nodeMetrics := make(map[string]*v1beta1.NodeMetrics)
	for i := range nodeMetricsList.Items {
		nodeMetrics[nodeMetricsList.Items[i].Name] = nodeMetricsList.Items[i].DeepCopy()
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
		nodeCPU := resource.MustParse("0")
		if nodeMetrics[pod.Spec.NodeName] != nil {
			nodeMem = nodeMetrics[pod.Spec.NodeName].Usage["memory"]
			nodeCPU = nodeMetrics[pod.Spec.NodeName].Usage["cpu"]
		}

		restarts := 0

		for _, containerStatus := range pod.Status.ContainerStatuses {
			restarts = restarts + int(containerStatus.RestartCount)
		}

		podHealth["timestamp"] = timestamp
		podHealth["retinaPod"] = pod.Name
		podHealth["podStatus"] = string(pod.Status.Phase)
		podHealth["podCpuInMilliCore"] = strconv.FormatInt(podCpu.MilliValue(), 10)
		podHealth["podMemoryInMB"] = strconv.FormatInt(podMem.Value()/(1048576), 10)
		podHealth["podRestarts"] = strconv.FormatInt(int64(restarts), 10)
		podHealth["retinaNode"] = pod.Spec.NodeName
		podHealth["nodeCpuInMilliCore"] = strconv.FormatInt(nodeCPU.MilliValue(), 10)
		podHealth["nodeMemoryInMB"] = strconv.FormatInt(nodeMem.Value()/(1048576), 10)

		allPodsHealth = append(allPodsHealth, podHealth)

	}

	return allPodsHealth, nil
}

func (g *GetAndPublishMetrics) SetAppInsightsKey(appInsightsKey string) *GetAndPublishMetrics {
	g.appInsightsKey = appInsightsKey
	return g
}

func (g *GetAndPublishMetrics) SetOutputFilePath(outputFilePath string) *GetAndPublishMetrics {
	g.outputFilePath = outputFilePath
	return g
}
