// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package steps

import (
	"context"
	"fmt"

	k8s "github.com/microsoft/retina/test/e2ev3/pkg/kubernetes"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// CurlPodStep executes a curl command from a source pod to a destination pod
// for flow testing. It resolves the destination pod's IP and runs the command.
type CurlPodStep struct {
	SrcPodName         string
	SrcPodNamespace    string
	DstPodName         string
	DstPodNamespace    string
	KubeConfigFilePath string
}

func (c *CurlPodStep) Do(_ context.Context) error {
	config, err := clientcmd.BuildConfigFromFlags("", c.KubeConfigFilePath)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("error creating Kubernetes client: %w", err)
	}

	dstPodIP, err := k8s.GetPodIP(c.KubeConfigFilePath, c.DstPodNamespace, c.DstPodName)
	if err != nil {
		return fmt.Errorf("error getting pod IP: %w", err)
	}

	cmd := fmt.Sprintf("curl -s -m 5 %s:80", dstPodIP)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err = k8s.ExecPod(ctx, clientset, config, c.SrcPodNamespace, c.SrcPodName, cmd)
	if err != nil {
		return fmt.Errorf("error executing command: %w", err)
	}
	return nil
}
