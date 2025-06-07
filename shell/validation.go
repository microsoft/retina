package shell

import (
	"context"
	"errors"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var errUnsupportedOperatingSystem = errors.New("unsupported OS (retina-shell requires Linux or Windows)")

// GetNodeOS retrieves the operating system of a node from its labels
func GetNodeOS(ctx context.Context, clientset kubernetes.Interface, nodeName string) (string, error) {
	node, err := clientset.CoreV1().
		Nodes().
		Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("error retrieving node %s: %w", nodeName, err)
	}

	return node.Labels["kubernetes.io/os"], nil
}

func validateOperatingSystemSupportedForNode(ctx context.Context, clientset kubernetes.Interface, nodeName string) error {
	os, err := GetNodeOS(ctx, clientset, nodeName)
	if err != nil {
		return err
	}

	// Support both Linux and Windows
	if os != "linux" && os != "windows" {
		return errUnsupportedOperatingSystem
	}

	return nil
}
