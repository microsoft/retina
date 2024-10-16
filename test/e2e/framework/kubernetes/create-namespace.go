package kubernetes

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type CreateNamespace struct {
	Namespace          string
	KubeConfigFilePath string
	DryRun             bool
}

func (c *CreateNamespace) Run() error {
	config, err := clientcmd.BuildConfigFromFlags("", c.KubeConfigFilePath)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("error creating Kubernetes client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeoutSeconds*time.Second)
	defer cancel()

	if !c.DryRun {
		_, err = clientset.CoreV1().Namespaces().Create(ctx, c.getNamespace(), metaV1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create namespace \"%s\": %w", c.Namespace, err)
		}
	}

	return nil
}

func (c *CreateNamespace) Stop() error {
	return nil
}

func (c *CreateNamespace) Prevalidate() error {
	return nil
}

func (c *CreateNamespace) getNamespace() *v1.Namespace {
	return &v1.Namespace{
		TypeMeta: metaV1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: "v1",
		},
		ObjectMeta: metaV1.ObjectMeta{
			Name: c.Namespace,
		},
	}
}
