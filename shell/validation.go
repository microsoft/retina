package shell

import (
	"context"
	"errors"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var errUnsupportedOperatingSystem = errors.New("unsupported OS (retina-shell requires Linux)")

func validateOperatingSystemSupportedForNode(ctx context.Context, clientset *kubernetes.Clientset, nodeName string) error {
	node, err := clientset.CoreV1().
		Nodes().
		Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error retrieving node %s: %w", nodeName, err)
	}

	osLabel := node.Labels["kubernetes.io/os"]
	if osLabel != "linux" { // Only Linux supported for now.
		return errUnsupportedOperatingSystem
	}

	return nil
}
