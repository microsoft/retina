// migrate from enterprise
package kubernetes

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type ResourceTypes string

const (
	ResourceTypePod     = "pod"
	ResourceTypeService = "service"
)

type ValidateResource struct {
	ResourceName       string
	ResourceNamespace  string
	ResourceType       string
	Labels             string
	KubeConfigFilePath string
}

func (v *ValidateResource) Run() error {
	config, err := clientcmd.BuildConfigFromFlags("", v.KubeConfigFilePath)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("error creating Kubernetes client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeoutSeconds*time.Second)
	defer cancel()

	switch v.ResourceType {
	case ResourceTypePod:
		err = WaitForPodReady(ctx, clientset, v.ResourceNamespace, v.Labels)
		if err != nil {
			return fmt.Errorf("pod not found: %w", err)
		}
	case ResourceTypeService:
		exists, err := serviceExists(ctx, clientset, v.ResourceNamespace, v.ResourceName, v.Labels)
		if err != nil || !exists {
			return fmt.Errorf("service not found: %w", err)
		}

	default:
		return fmt.Errorf("resource type %s not supported", v.ResourceType)
	}

	if err != nil {
		return fmt.Errorf("error waiting for pod to be ready: %w", err)
	}
	return nil
}

func serviceExists(ctx context.Context, clientset *kubernetes.Clientset, namespace, serviceName, labels string) (bool, error) {
	var serviceList *corev1.ServiceList
	serviceList, err := clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{LabelSelector: labels})
	if err != nil {
		return false, fmt.Errorf("error listing Services: %w", err)
	}
	if len(serviceList.Items) == 0 {
		return false, nil
	}
	return true, nil
}

func (v *ValidateResource) Prevalidate() error {
	return nil
}

func (v *ValidateResource) Stop() error {
	return nil
}
