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
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
)

type DeleteNamespace struct {
	Namespace  string
	RestConfig *rest.Config
}

func (d *DeleteNamespace) String() string { return "delete-namespace" }

func (d *DeleteNamespace) Do(ctx context.Context) error {
	clientset, err := kubernetes.NewForConfig(d.RestConfig)
	if err != nil {
		return fmt.Errorf("error creating Kubernetes client: %w", err)
	}

	err = clientset.CoreV1().Namespaces().Delete(ctx, d.Namespace, metaV1.DeleteOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to delete namespace \"%s\": %w", d.Namespace, err)
		}
	}

	backoff := wait.Backoff{
		Steps:    9,
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
