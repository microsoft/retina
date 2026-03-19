// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package advancedmetrics

import (
	"time"

	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/common"
	k8s "github.com/microsoft/retina/test/e2ev3/framework/kubernetes"
	"github.com/microsoft/retina/test/e2ev3/steps"
)

func addLatencyScenario(wf *flow.Workflow, upstream flow.Steper, kubeConfigFilePath string) flow.Steper {
	sleep5 := &steps.SleepStep{Duration: 5 * time.Second}
	pf := &k8s.PortForward{
		Namespace: common.KubeSystemNamespace, LabelSelector: "k8s-app=retina",
		LocalPort: "10093", RemotePort: "10093", Endpoint: "metrics",
		KubeConfigFilePath: kubeConfigFilePath, OptionalLabelAffinity: "k8s-app=retina",
	}
	validateLatency := &steps.ValidateAPIServerLatencyStep{}
	stopPF := &steps.StopPortForwardStep{PF: pf}

	chain := []flow.Steper{sleep5, pf, validateLatency, stopPF}
	wf.Add(flow.Pipe(chain...).DependsOn(upstream))
	return stopPF
}
