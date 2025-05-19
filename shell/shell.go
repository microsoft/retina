package shell

import (
	"context"
	"fmt"
	"os"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Config is the configuration for starting a shell in a node or pod.
type Config struct {
	RestConfig       *rest.Config
	RetinaShellImage string
	HostPID          bool
	Capabilities     []string
	Timeout          time.Duration
	NodeOS           string // "linux" or "windows"

	// Host filesystem access applies only to nodes, not pods.
	MountHostFilesystem      bool
	AllowHostFilesystemWrite bool
}

// RunInPod starts an interactive shell in a pod by creating and attaching to an ephemeral container.
func RunInPod(config Config, podNamespace, podName string) error {
	ctx := context.Background()

	clientset, err := kubernetes.NewForConfig(config.RestConfig)
	if err != nil {
		return fmt.Errorf("error constructing kube clientset: %w", err)
	}

	pod, err := clientset.CoreV1().
		Pods(podNamespace).
		Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error retrieving pod %s from namespace %s: %w", podName, podNamespace, err)
	}

	err = validateOperatingSystemSupportedForNode(ctx, clientset, pod.Spec.NodeName)
	if err != nil {
		return fmt.Errorf("error validating operating system for node %s: %w", pod.Spec.NodeName, err)
	}

	// Get the node OS for the pod's node
	nodeOS, err := GetNodeOS(ctx, clientset, pod.Spec.NodeName)
	if err != nil {
		return fmt.Errorf("error getting OS for node %s: %w", pod.Spec.NodeName, err)
	}
	config.NodeOS = nodeOS

	fmt.Printf("Starting ephemeral container in pod %s/%s\n", podNamespace, podName)
	ephemeralContainer := ephemeralContainerForPodDebug(config)
	pod.Spec.EphemeralContainers = append(pod.Spec.EphemeralContainers, ephemeralContainer)

	_, err = clientset.CoreV1().
		Pods(podNamespace).
		UpdateEphemeralContainers(ctx, podName, pod, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("error updating ephemeral containers: %w", err)
	}

	if err := waitForContainerRunning(ctx, config.Timeout, clientset, podNamespace, podName, ephemeralContainer.Name); err != nil {
		return fmt.Errorf("error waiting for containers running: %w", err)
	}

	return attachToShell(config.RestConfig, podNamespace, podName, ephemeralContainer.Name, pod)
}

// RunInNode starts an interactive shell on a node by creating a HostNetwork pod and attaching to it.
func RunInNode(config Config, nodeName, debugPodNamespace string) error {
	ctx := context.Background()

	clientset, err := kubernetes.NewForConfig(config.RestConfig)
	if err != nil {
		return fmt.Errorf("error constructing kube clientset: %w", err)
	}

	err = validateOperatingSystemSupportedForNode(ctx, clientset, nodeName)
	if err != nil {
		return fmt.Errorf("error validating operating system for node %s: %w", nodeName, err)
	}

	// Get the node OS
	nodeOS, err := GetNodeOS(ctx, clientset, nodeName)
	if err != nil {
		return fmt.Errorf("error getting OS for node %s: %w", nodeName, err)
	}
	config.NodeOS = nodeOS

	pod := hostNetworkPodForNodeDebug(config, debugPodNamespace, nodeName)

	fmt.Printf("Starting host networking pod %s/%s on node %s\n", debugPodNamespace, pod.Name, nodeName)
	_, err = clientset.CoreV1().
		Pods(debugPodNamespace).
		Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("err creating pod %s in namespace %s: %w", pod.Name, debugPodNamespace, err)
	}

	defer func() {
		// Best-effort cleanup.
		err = clientset.CoreV1().
			Pods(debugPodNamespace).
			Delete(ctx, pod.Name, metav1.DeleteOptions{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to delete pod %s: %v\n", pod.Name, err)
		}
	}()

	err = waitForContainerRunning(ctx, config.Timeout, clientset, debugPodNamespace, pod.Name, pod.Spec.Containers[0].Name)
	if err != nil {
		return err
	}

	return attachToShell(config.RestConfig, debugPodNamespace, pod.Name, pod.Spec.Containers[0].Name, pod)
}
