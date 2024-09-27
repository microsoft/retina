package kubernetes

import (
	"context"
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var ErrPodCrashed = fmt.Errorf("pod has crashes")

type EnsureStableCluster struct {
	LabelSelector      string
	PodNamespace       string
	KubeConfigFilePath string
}

func (n *EnsureStableCluster) Run() error {
	config, err := clientcmd.BuildConfigFromFlags("", n.KubeConfigFilePath)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("error creating Kubernetes client: %w", err)
	}

	err = WaitForPodReady(context.TODO(), clientset, n.PodNamespace, n.LabelSelector)
	if err != nil {
		return fmt.Errorf("error waiting for retina pods to be ready: %w", err)
	}
	return nil
}

func (n *EnsureStableCluster) Prevalidate() error {
	return nil
}

func (n *EnsureStableCluster) Stop() error {
	return nil
}
