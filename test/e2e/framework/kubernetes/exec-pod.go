package kubernetes

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/scheme"
)

const ExecSubResources = "exec"

type ExecInPod struct {
	PodNamespace       string
	KubeConfigFilePath string
	PodName            string
	Command            string
}

func (e *ExecInPod) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := ExecPod(ctx, e.KubeConfigFilePath, e.PodNamespace, e.PodName, e.Command)
	if err != nil {
		return fmt.Errorf("error executing command [%s]: %w", e.Command, err)
	}

	return nil
}

func (e *ExecInPod) Prevalidate() error {
	return nil
}

func (e *ExecInPod) Stop() error {
	return nil
}

func ExecPod(ctx context.Context, kubeConfigFilePath, namespace, podName, command string) error {
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigFilePath)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("error creating Kubernetes client: %w", err)
	}

	req := clientset.CoreV1().RESTClient().Post().Resource("pods").Name(podName).
		Namespace(namespace).SubResource(ExecSubResources)
	option := &v1.PodExecOptions{
		Command: strings.Fields(command),
		Stdin:   true,
		Stdout:  true,
		Stderr:  true,
		TTY:     false,
	}

	req.VersionedParams(
		option,
		scheme.ParameterCodec,
	)

	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("error creating executor: %w", err)
	}

	log.Printf("executing command \"%s\" on pod \"%s\" in namespace \"%s\"...", command, podName, namespace)
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	})
	if err != nil {
		return fmt.Errorf("error executing command: %w", err)
	}

	return nil
}
