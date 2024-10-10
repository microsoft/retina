package kubernetes

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func CreateNamespace(kubeconfigpath, namespace string) error {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigpath)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("error creating Kubernetes client: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, err = clientset.CoreV1().Namespaces().Create(ctx, &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}, metav1.CreateOptions{})

	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("error creating namespace: %w", err)
	} else {
		fmt.Printf("Namespace '%s' created successfully.\n", namespace)
		return nil
	}
}
