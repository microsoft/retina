package scaletest

import (
	"context"
	"fmt"
	"log"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	platformPodValidationTimeout  = 5 * time.Minute
	platformPodValidationInterval = 5 * time.Second
)

type ValidatePlatformPods struct {
	kubeConfigFilePath  string
	namespace           string
	labelSelector       string
	ExpectedLinuxPods   int
	ExpectedWindowsPods int
	RequireNoRestarts   bool
}

func NewValidatePlatformPods(kubeConfigFilePath, namespace, labelSelector string) *ValidatePlatformPods {
	return &ValidatePlatformPods{
		kubeConfigFilePath: kubeConfigFilePath,
		namespace:          namespace,
		labelSelector:      labelSelector,
	}
}

func (v *ValidatePlatformPods) Prevalidate() error {
	if v.ExpectedLinuxPods < 0 || v.ExpectedWindowsPods < 0 {
		return fmt.Errorf("expected pod counts must not be negative")
	}
	return nil
}

func (v *ValidatePlatformPods) Run() error {
	config, err := clientcmd.BuildConfigFromFlags("", v.kubeConfigFilePath)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("error creating Kubernetes client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), platformPodValidationTimeout)
	defer cancel()

	var linux, windows, unknown int
	err = wait.PollUntilContextCancel(ctx, platformPodValidationInterval, true, func(ctx context.Context) (bool, error) {
		pods, err := clientset.CoreV1().Pods(v.namespace).List(ctx, metav1.ListOptions{LabelSelector: v.labelSelector})
		if err != nil {
			return false, fmt.Errorf("error listing pods: %w", err)
		}

		nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			return false, fmt.Errorf("error listing nodes: %w", err)
		}

		var restarts int
		linux, windows, unknown, restarts = countPlatformPods(pods.Items, nodes.Items)
		if v.RequireNoRestarts && restarts > 0 {
			return false, fmt.Errorf("pods with label %q have %d container restarts", v.labelSelector, restarts)
		}

		complete := linux == v.ExpectedLinuxPods &&
			windows == v.ExpectedWindowsPods &&
			unknown == 0
		if !complete {
			log.Printf(
				"waiting for pod distribution with label %q: got linux=%d windows=%d unknown=%d, want linux=%d windows=%d",
				v.labelSelector,
				linux,
				windows,
				unknown,
				v.ExpectedLinuxPods,
				v.ExpectedWindowsPods,
			)
		}
		return complete, nil
	})
	if err != nil {
		return fmt.Errorf(
			"unexpected pod distribution for label %q: got linux=%d windows=%d unknown=%d, want linux=%d windows=%d: %w",
			v.labelSelector,
			linux,
			windows,
			unknown,
			v.ExpectedLinuxPods,
			v.ExpectedWindowsPods,
			err,
		)
	}

	return nil
}

func (v *ValidatePlatformPods) Stop() error {
	return nil
}

func countPlatformPods(pods []corev1.Pod, nodes []corev1.Node) (linux, windows, unknown, restarts int) {
	nodePlatforms := make(map[string]string, len(nodes))
	for i := range nodes {
		os := nodes[i].Labels[corev1.LabelOSStable]
		arch := nodes[i].Labels[corev1.LabelArchStable]
		if arch == "amd64" && (os == "linux" || os == "windows") {
			nodePlatforms[nodes[i].Name] = os
		}
	}

	for i := range pods {
		switch nodePlatforms[pods[i].Spec.NodeName] {
		case "linux":
			linux++
		case "windows":
			windows++
		default:
			unknown++
		}

		for j := range pods[i].Status.ContainerStatuses {
			restarts += int(pods[i].Status.ContainerStatuses[j].RestartCount)
		}
	}

	return linux, windows, unknown, restarts
}
