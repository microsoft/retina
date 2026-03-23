package kubernetes

import (
	"context"
	"fmt"
	"log/slog"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type CreateNamespace struct {
	Namespace  string
	RestConfig *rest.Config
}

func (c *CreateNamespace) String() string { return "create-namespace" }

func (c *CreateNamespace) Do(ctx context.Context) error {
	return CreateNamespaceFn(ctx, c.RestConfig, c.Namespace)
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

func CreateNamespaceFn(ctx context.Context, restConfig *rest.Config, namespace string) error {
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("error creating Kubernetes client: %w", err)
	}

	_, err = clientset.CoreV1().Namespaces().Create(ctx, &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create namespace \"%s\": %w", namespace, err)
	}

	slog.Info("namespace created", "namespace", namespace)

	return nil
}
