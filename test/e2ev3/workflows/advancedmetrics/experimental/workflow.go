// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package experimental

import (
	"context"

	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/config"
	k8s "github.com/microsoft/retina/test/e2ev3/pkg/kubernetes"
	"github.com/microsoft/retina/test/e2ev3/pkg/utils"
)

// Workflow runs the experimental advanced metrics workflow.
type Workflow struct {
	Cfg *config.E2EConfig
}

func (w *Workflow) String() string { return "advanced-metrics-experimental" }

func (w *Workflow) Do(ctx context.Context) error {
	p := w.Cfg
	restConfig := p.Cluster.RestConfig()
	chartPath := p.Paths.RetinaChart
	valuesFilePath := p.Paths.AdvancedProfile
	testPodNamespace := config.TestPodNamespace
	helmCfg := &p.Helm

	// Construct steps.
	upgradeRetina := &k8s.UpgradeRetinaHelmChart{
		Namespace:          config.KubeSystemNamespace,
		ReleaseName:        "retina",
		KubeConfigFilePath: p.Cluster.KubeConfigPath(),
		ChartPath:          chartPath,
		HelmDriver:         helmCfg.Driver,
		ValuesFile:         valuesFilePath,
	}

	var scenarios []flow.Steper
	for _, arch := range config.Architectures {
		scenarios = append(scenarios,
			addAdvancedDropScenario(restConfig, testPodNamespace, arch),
			addAdvancedForwardScenario(restConfig, testPodNamespace, arch),
			addAdvancedTCPScenario(restConfig, testPodNamespace, arch),
		)
	}
	scenarios = append(scenarios, addAPIServerLatencyScenario(restConfig))

	ensureStable := &k8s.EnsureStableComponent{
		PodNamespace:           config.KubeSystemNamespace,
		LabelSelector:          "k8s-app=retina",
		RestConfig:             restConfig,
		IgnoreContainerRestart: false,
	}

	debug := &utils.DebugOnFailure{
		RestConfig: restConfig,
		Namespace:          config.KubeSystemNamespace,
		LabelSelector:      "k8s-app=retina",
	}

	// Wire dependencies and register.
	// Scenarios run sequentially because they share the same port-forward port.
	wf := &flow.Workflow{DontPanic: true}
	wf.Add(flow.Step(upgradeRetina))
	prev := flow.Steper(upgradeRetina)
	for _, s := range scenarios {
		wf.Add(flow.Step(s).DependsOn(prev))
		prev = s
	}
	wf.Add(flow.Step(ensureStable).DependsOn(prev))
	wf.Add(flow.Step(debug).DependsOn(ensureStable).When(flow.AnyFailed))

	return wf.Do(ctx)
}
