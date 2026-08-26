package scaletest

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	nodeValidationTimeout  = 30 * time.Minute
	nodeValidationInterval = 15 * time.Second
)

type ValidateNumOfNodes struct {
	NumNodesRequired   int
	Label              map[string]string
	KubeConfigFilePath string
}

// Useful when wanting to do parameter checking, for example
// if a parameter length is known to be required less than 80 characters,
// do this here so we don't find out later on when we run the step
// when possible, try to avoid making external calls, this should be fast and simple
func (v *ValidateNumOfNodes) Prevalidate() error {
	return nil
}

// Primary step where test logic is executed
// Returning an error will cause the test to fail
func (v *ValidateNumOfNodes) Run() error {
	config, err := clientcmd.BuildConfigFromFlags("", v.KubeConfigFilePath)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("error creating Kubernetes client: %w", err)
	}

	labelSelector := labels.Set(v.Label).String()
	ctx, cancel := context.WithTimeout(context.Background(), nodeValidationTimeout)
	defer cancel()

	var matchingNodes, readyNodes int
	var lastListErr error
	err = wait.PollUntilContextCancel(ctx, nodeValidationInterval, true, func(ctx context.Context) (bool, error) {
		nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
		if err != nil {
			lastListErr = errors.Wrap(err, "error getting nodes")
			log.Printf("retrying node validation after error: %v", lastListErr)
			return false, nil
		}
		lastListErr = nil

		matchingNodes = len(nodes.Items)
		readyNodes = countReadyNodes(nodes.Items)
		if readyNodes < v.NumNodesRequired {
			log.Printf(
				"waiting for %d Ready nodes matching %q: got %d Ready out of %d matching nodes",
				v.NumNodesRequired,
				labelSelector,
				readyNodes,
				matchingNodes,
			)
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		if lastListErr != nil {
			return fmt.Errorf(
				"need %d Ready nodes matching %q, got %d Ready out of %d matching nodes before timeout (last list error: %v): %w",
				v.NumNodesRequired,
				labelSelector,
				readyNodes,
				matchingNodes,
				lastListErr,
				err,
			)
		}
		return fmt.Errorf(
			"need %d Ready nodes matching %q, got %d Ready out of %d matching nodes before timeout: %w",
			v.NumNodesRequired,
			labelSelector,
			readyNodes,
			matchingNodes,
			err,
		)
	}

	return nil
}

// Require for background steps
func (v *ValidateNumOfNodes) Stop() error {
	return nil
}

func countReadyNodes(nodes []corev1.Node) int {
	ready := 0
	for i := range nodes {
		for _, condition := range nodes[i].Status.Conditions {
			if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
				ready++
				break
			}
		}
	}
	return ready
}
