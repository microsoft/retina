package shell

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/cmd/attach"
	"k8s.io/kubectl/pkg/cmd/exec"
)

func attachToShell(restConfig *rest.Config, namespace, podName, containerName string, pod *v1.Pod) error {
	attachOpts := &attach.AttachOptions{
		Config: restConfig,
		StreamOptions: exec.StreamOptions{
			Namespace:     namespace,
			PodName:       podName,
			ContainerName: containerName,
			IOStreams: genericiooptions.IOStreams{
				In:     os.Stdin,
				Out:    os.Stdout,
				ErrOut: os.Stderr,
			},
			Stdin: true,
			TTY:   true,
			Quiet: true,
		},
		Attach:     &attach.DefaultRemoteAttach{},
		AttachFunc: attach.DefaultAttachFunc,
		Pod:        pod,
	}

	if err := attachOpts.Run(); err != nil {
		return fmt.Errorf("error attaching to shell container: %w", err)
	}

	return nil
}

func waitForContainerRunning(ctx context.Context, timeout time.Duration, clientset *kubernetes.Clientset, namespace, podName, containerName string) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	for {
		pod, err := clientset.CoreV1().
			Pods(namespace).
			Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				return waitTimeoutError(err, timeout, containerName)
			}
			return fmt.Errorf("error retrieving pod %s in namespace %s: %w", podName, namespace, err)
		}

		for i := range pod.Status.ContainerStatuses {
			status := pod.Status.ContainerStatuses[i]
			if status.Name == containerName && status.State.Running != nil {
				return nil
			}
		}
		for i := range pod.Status.EphemeralContainerStatuses {
			status := pod.Status.EphemeralContainerStatuses[i]
			if status.Name == containerName && status.State.Running != nil {
				return nil
			}
		}

		select {
		case <-ctx.Done():
			return waitTimeoutError(context.DeadlineExceeded, timeout, containerName)
		case <-time.After(1 * time.Second):
		}
	}
}

func waitTimeoutError(err error, timeout time.Duration, containerName string) error {
	return fmt.Errorf("timed out after %s waiting for container %s to start. The timeout can be increased by setting --timeout. Err: %w", timeout, containerName, err)
}
