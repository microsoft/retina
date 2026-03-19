// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package advancedmetrics

import (
	"time"

	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/common"
	k8s "github.com/microsoft/retina/test/e2ev3/pkg/kubernetes"
	"github.com/microsoft/retina/test/e2ev3/steps"
)

func addLatencyScenario(wf *flow.Workflow, upstream flow.Steper, kubeConfigFilePath string) flow.Steper {
	validateLatency := &steps.ValidateAPIServerLatencyStep{}
	validateWithPF := &steps.WithPortForward{
		PF: &k8s.PortForward{
			Namespace: common.KubeSystemNamespace, LabelSelector: "k8s-app=retina",
			LocalPort: "10093", RemotePort: "10093", Endpoint: "metrics",
			KubeConfigFilePath: kubeConfigFilePath, OptionalLabelAffinity: "k8s-app=retina",
		},
		Steps: []flow.Steper{validateLatency},
	}

	wf.Add(flow.Step(validateWithPF).DependsOn(upstream).Retry(steps.RetryValidation()...).Timeout(5 * time.Minute))
	return validateWithPF
}
