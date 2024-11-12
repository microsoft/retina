package kubernetes

import (
	"context"
	"fmt"
	"log"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/retry"
)

type DeleteNamespace struct {
	Namespace          string
	KubeConfigFilePath string
}

func (d *DeleteNamespace) Run() error {
	config, err := clientcmd.BuildConfigFromFlags("", d.KubeConfigFilePath)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("error creating Kubernetes client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeoutSeconds*time.Second)
	defer cancel()

	err = clientset.CoreV1().Namespaces().Delete(ctx, d.Namespace, metaV1.DeleteOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to delete namespace \"%s\": %w", d.Namespace, err)
		}
	}

	backoff := wait.Backoff{
		Steps:    6,
		Duration: 10 * time.Second,
		Factor:   2.0,
		// Jitter:   0.1,
	}

	// Check if namespace was deleted
	return retry.OnError(backoff,
		func(err error) bool {
			log.Printf("%v. Checking again soon...", err)

			return true
		},
		func() error {
			_, err = clientset.CoreV1().Namespaces().Get(ctx, d.Namespace, metaV1.GetOptions{})
			if errors.IsNotFound(err) {
				return nil
			}

			if err == nil {
				return fmt.Errorf("namespace \"%s\" still exists", d.Namespace)
			}

			return err
		},
	)
}

func (d *DeleteNamespace) Stop() error {
	return nil
}

func (d *DeleteNamespace) Prevalidate() error {
	return nil
}
