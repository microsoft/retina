package scaletest

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
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

	ctx := context.TODO()

	labelSelector := labels.Set(v.Label).String()
	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return errors.Wrap(err, "error getting nodes")
	}

	if len(nodes.Items) < v.NumNodesRequired {
		return fmt.Errorf("need %d real nodes to achieve the required max number of pods per node, got %d. Make sure to label nodes with: kubectl label node <name> %s", v.NumNodesRequired, len(nodes.Items), labelSelector)
	}

	return nil
}

// Require for background steps
func (v *ValidateNumOfNodes) Stop() error {
	return nil
}
