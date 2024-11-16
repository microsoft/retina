package kubernetes

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type CreateNamespace struct {
	Namespace          string
	KubeConfigFilePath string
}

func (c *CreateNamespace) Run() error {
	return CreateNamespaceFn(c.KubeConfigFilePath, c.Namespace)
}

func (c *CreateNamespace) Stop() error {
	return nil
}

func (c *CreateNamespace) Prevalidate() error {
	return nil
}

func (c *CreateNamespace) getNamespace() *v1.Namespace {
	return &v1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: c.Namespace,
		},
	}
}

func CreateNamespaceFn(kubeconfigpath, namespace string) error {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigpath)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("error creating Kubernetes client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeoutSeconds*time.Second)
	defer cancel()

	_, err = clientset.CoreV1().Namespaces().Create(ctx, &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create namespace \"%s\": %w", namespace, err)
	}

	fmt.Printf("Namespace \"%s\" created.\n", namespace)

	return nil
}
