package kubernetes

import (
	"context"
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var ErrPodCrashed = fmt.Errorf("pod has crashes")

type EnsureStableComponent struct {
	LabelSelector      string
	PodNamespace       string
	KubeConfigFilePath string

	// Container restarts can occur for various reason, they do not necessarily mean the entire cluster
	// is unstable or needs to be recreated. In some cases, container restarts are expected and acceptable.
	// This flag should be set to true only in those cases and provide additional why restart restarts are acceptable.
	IgnoreContainerRestart bool
}

func (n *EnsureStableComponent) Run() error {
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

	if !n.IgnoreContainerRestart {
		err = CheckContainerRestart(context.TODO(), clientset, n.PodNamespace, n.LabelSelector)
		if err != nil {
			return fmt.Errorf("error checking pod restarts: %w", err)
		}
	}
	return nil
}

func (n *EnsureStableComponent) Prevalidate() error {
	return nil
}

func (n *EnsureStableComponent) Stop() error {
	return nil
}
