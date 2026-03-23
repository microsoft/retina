// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package hubblemetrics

import (
	"context"
	"fmt"
	"log/slog"

	k8s "github.com/microsoft/retina/test/e2ev3/pkg/kubernetes"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"github.com/microsoft/retina/test/e2ev3/pkg/stepname"
)

// CurlPodStep executes a curl command from a source pod to a destination pod
// for flow testing. It resolves the destination pod's IP and runs the command.
type CurlPodStep struct {
	SrcPodName      string
	SrcPodNamespace string
	DstPodName      string
	DstPodNamespace string
	RestConfig      *rest.Config
	Log             *slog.Logger
}

func (c *CurlPodStep) Do(ctx context.Context) error {
	log := c.Log
	if log == nil {
		log = slog.Default()
	}
	log = log.With("step", stepname.StepName(c))
	clientset, err := kubernetes.NewForConfig(c.RestConfig)
	if err != nil {
		return fmt.Errorf("error creating Kubernetes client: %w", err)
	}

	dstPodIP, err := k8s.GetPodIP(ctx, c.RestConfig, c.DstPodNamespace, c.DstPodName)
	if err != nil {
		return fmt.Errorf("error getting pod IP: %w", err)
	}

	cmd := fmt.Sprintf("curl -s -m 5 %s:80", dstPodIP)
	_, err = k8s.ExecPod(ctx, clientset, c.RestConfig, c.SrcPodNamespace, c.SrcPodName, cmd)
	if err != nil {
		return fmt.Errorf("error executing command: %w", err)
	}
	return nil
}
