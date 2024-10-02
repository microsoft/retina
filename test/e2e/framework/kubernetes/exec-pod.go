package kubernetes

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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

	config, err := clientcmd.BuildConfigFromFlags("", e.KubeConfigFilePath)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("error creating Kubernetes client: %w", err)
	}

	_, err = ExecPod(ctx, clientset, config, e.PodNamespace, e.PodName, e.Command)
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

func ExecPod(ctx context.Context, clientset *kubernetes.Clientset, config *rest.Config, namespace, podName, command string) ([]byte, error) {
	log.Printf("executing command \"%s\" on pod \"%s\" in namespace \"%s\"...", command, podName, namespace)
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

	var buf bytes.Buffer
	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return buf.Bytes(), fmt.Errorf("error creating executor: %w", err)
	}

	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  os.Stdin,
		Stdout: &buf,
		Stderr: &buf,
	})
	if err != nil {
		return buf.Bytes(), fmt.Errorf("error executing command: %w", err)
	}

	res := buf.Bytes()
	return res, nil
}
