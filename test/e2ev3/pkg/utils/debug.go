// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package utils

import (
	"context"
	"log/slog"

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

func (d *DebugOnFailure) String() string { return "debug-on-failure" }

func (d *DebugOnFailure) Do(_ context.Context) error {
	slog.Info("capturing logs for pods", "namespace", d.Namespace, "label", d.LabelSelector)
	getLogs := &k8s.GetPodLogs{
		RestConfig:    d.RestConfig,
		Namespace:          d.Namespace,
		LabelSelector:      d.LabelSelector,
	}
	if err := getLogs.Do(context.Background()); err != nil {
		slog.Error("failed to capture logs", "error", err)
	}
	return nil // never fail the debug step itself
}
