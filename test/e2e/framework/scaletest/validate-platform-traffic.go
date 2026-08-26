package scaletest

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	trafficValidationTimeout  = 2 * time.Minute
	trafficValidationInterval = 10 * time.Second
)

var responseServiceIP = regexp.MustCompile(`response from http://([^:/]+):`)

type ValidatePlatformTraffic struct {
	kubeConfigFilePath string
	namespace          string
	labelSelector      string
}

func NewValidatePlatformTraffic(kubeConfigFilePath, namespace, labelSelector string) *ValidatePlatformTraffic {
	return &ValidatePlatformTraffic{
		kubeConfigFilePath: kubeConfigFilePath,
		namespace:          namespace,
		labelSelector:      labelSelector,
	}
}

func (v *ValidatePlatformTraffic) Prevalidate() error {
	return nil
}

func (v *ValidatePlatformTraffic) Run() error {
	config, err := clientcmd.BuildConfigFromFlags("", v.kubeConfigFilePath)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("error creating Kubernetes client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), trafficValidationTimeout)
	defer cancel()

	serviceTargets, err := getServiceTargets(ctx, clientset, v.namespace, v.labelSelector)
	if err != nil {
		return err
	}

	sourcePods, err := getPlatformSourcePods(ctx, clientset, v.namespace)
	if err != nil {
		return err
	}

	observed := map[string]bool{}
	err = wait.PollUntilContextCancel(ctx, trafficValidationInterval, true, func(ctx context.Context) (bool, error) {
		for sourceOS, pod := range sourcePods {
			logs, err := clientset.CoreV1().Pods(v.namespace).GetLogs(pod, &corev1.PodLogOptions{
				Container: "kapinger",
			}).DoRaw(ctx)
			if err != nil {
				log.Printf("error getting logs from %s source pod %s: %v", sourceOS, pod, err)
				continue
			}

			for targetOS := range observedTrafficTargets(string(logs), serviceTargets) {
				observed[sourceOS+"->"+targetOS] = true
			}
		}

		return observed["linux->linux"] &&
			observed["linux->windows"] &&
			observed["windows->linux"] &&
			observed["windows->windows"], nil
	})
	if err != nil {
		return fmt.Errorf("did not observe all mixed-platform traffic directions before timeout (observed: %v): %w", observed, err)
	}

	return nil
}

func (v *ValidatePlatformTraffic) Stop() error {
	return nil
}

func getServiceTargets(ctx context.Context, clientset kubernetes.Interface, namespace, labelSelector string) (map[string]string, error) {
	services, err := clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return nil, fmt.Errorf("error listing traffic services: %w", err)
	}

	targets := make(map[string]string, len(services.Items))
	platforms := map[string]bool{}
	for i := range services.Items {
		os := services.Items[i].Labels[TrafficOSLabel]
		if os != "linux" && os != "windows" {
			continue
		}
		targets[services.Items[i].Spec.ClusterIP] = os
		platforms[os] = true
	}
	if !platforms["linux"] || !platforms["windows"] {
		return nil, fmt.Errorf("traffic services must include both Linux and Windows targets")
	}

	return targets, nil
}

func getPlatformSourcePods(ctx context.Context, clientset kubernetes.Interface, namespace string) (map[string]string, error) {
	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: "is-real=true"})
	if err != nil {
		return nil, fmt.Errorf("error listing traffic pods: %w", err)
	}
	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("error listing traffic nodes: %w", err)
	}

	nodeOS := make(map[string]string, len(nodes.Items))
	for i := range nodes.Items {
		nodeOS[nodes.Items[i].Name] = nodes.Items[i].Labels[corev1.LabelOSStable]
	}

	sources := make(map[string]string, 2)
	for i := range pods.Items {
		os := nodeOS[pods.Items[i].Spec.NodeName]
		if (os == "linux" || os == "windows") && sources[os] == "" {
			sources[os] = pods.Items[i].Name
		}
	}
	if sources["linux"] == "" || sources["windows"] == "" {
		return nil, fmt.Errorf("traffic pods must include both Linux and Windows sources")
	}

	return sources, nil
}

func observedTrafficTargets(logs string, serviceTargets map[string]string) map[string]bool {
	observed := make(map[string]bool, 2)
	for _, match := range responseServiceIP.FindAllStringSubmatch(logs, -1) {
		if os := serviceTargets[match[1]]; os != "" {
			observed[os] = true
		}
	}
	return observed
}
