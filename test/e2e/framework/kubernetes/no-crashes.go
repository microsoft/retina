package kubernetes

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type NoCrashes struct {
	LabelSelector      string
	PodNamespace       string
	KubeConfigFilePath string
}

func (n *NoCrashes) Run() error {
	config, err := clientcmd.BuildConfigFromFlags("", n.KubeConfigFilePath)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("error creating Kubernetes client: %w", err)
	}

	fieldSelector := fields.Everything()

	pods, err := clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{
		LabelSelector: n.LabelSelector,
		FieldSelector: fieldSelector.String(),
	})
	if err != nil {
		return fmt.Errorf("error listing pods: %w", err)
	}

	for _, pod := range pods.Items {
		for _, status := range pod.Status.ContainerStatuses {
			if status.RestartCount > 0 {
				PrintPodLogs(n.KubeConfigFilePath, pod.Namespace, pod.Name)
				return fmt.Errorf("Pod %s has %d restarts", pod.Name, status)
			}
		}
	}

	return nil
}

func (n *NoCrashes) Prevalidate() error {
	return nil
}

func (n *NoCrashes) Stop() error {
	return nil
}
