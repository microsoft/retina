// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package utils

import (
	"context"
	"log"

	k8s "github.com/microsoft/retina/test/e2ev3/pkg/kubernetes"
	"k8s.io/client-go/rest"
)

// DebugOnFailure captures diagnostic info when upstream steps fail.
// Add it to a workflow with When(flow.AnyFailed) so it only runs on failure.
type DebugOnFailure struct {
	RestConfig    *rest.Config
	Namespace     string
	LabelSelector string
}

func (d *DebugOnFailure) Do(_ context.Context) error {
	log.Printf("[DEBUG] Capturing logs for pods in %s with label %s", d.Namespace, d.LabelSelector)
	getLogs := &k8s.GetPodLogs{
		RestConfig:    d.RestConfig,
		Namespace:          d.Namespace,
		LabelSelector:      d.LabelSelector,
	}
	if err := getLogs.Do(context.Background()); err != nil {
		log.Printf("[DEBUG] Failed to capture logs: %v", err)
	}
	return nil // never fail the debug step itself
}
