package kubernetes

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/microsoft/retina/test/e2ev3/pkg/stepname"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type CreateNamespace struct {
	Namespace  string
	RestConfig *rest.Config
	Log        *slog.Logger
}

func (c *CreateNamespace) Do(ctx context.Context) error {
	log := c.Log
	if log == nil {
		log = slog.Default()
	}
	log = log.With("step", stepname.StepName(c))
	return CreateNamespaceFn(ctx, log, c.RestConfig, c.Namespace)
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

func CreateNamespaceFn(ctx context.Context, log *slog.Logger, restConfig *rest.Config, namespace string) error {
	if log == nil {
		log = slog.Default()
	}
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

	log.Info("namespace created", "namespace", namespace)

	return nil
}
