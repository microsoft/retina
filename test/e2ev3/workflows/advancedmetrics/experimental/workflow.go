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
	Params *config.E2EParams
}

func (w *Workflow) String() string { return "advanced-metrics-experimental" }

func (w *Workflow) Do(ctx context.Context) error {
	p := w.Params
	kubeConfigFilePath := p.Paths.KubeConfig
	chartPath := p.Paths.RetinaChart
	valuesFilePath := p.Paths.AdvancedProfile
	testPodNamespace := config.TestPodNamespace
	helmCfg := &p.Cfg.Helm

	// Construct steps.
	upgradeRetina := &k8s.UpgradeRetinaHelmChart{
		Namespace:          config.KubeSystemNamespace,
		ReleaseName:        "retina",
		KubeConfigFilePath: kubeConfigFilePath,
		ChartPath:          chartPath,
		HelmDriver:         helmCfg.Driver,
		ValuesFile:         valuesFilePath,
	}

	var scenarioTails []flow.Steper
	for _, arch := range config.Architectures {
		scenarioTails = append(scenarioTails,
			addAdvancedDropScenario(kubeConfigFilePath, testPodNamespace, arch),
			addAdvancedForwardScenario(kubeConfigFilePath, testPodNamespace, arch),
			addAdvancedTCPScenario(kubeConfigFilePath, testPodNamespace, arch),
		)
	}
	scenarioTails = append(scenarioTails, addAPIServerLatencyScenario(kubeConfigFilePath))

	ensureStable := &k8s.EnsureStableComponent{
		PodNamespace:           config.KubeSystemNamespace,
		LabelSelector:          "k8s-app=retina",
		KubeConfigFilePath:     kubeConfigFilePath,
		IgnoreContainerRestart: false,
	}

	debug := &utils.DebugOnFailure{
		KubeConfigFilePath: kubeConfigFilePath,
		Namespace:          config.KubeSystemNamespace,
		LabelSelector:      "k8s-app=retina",
	}

	// Wire dependencies and register.
	wf := &flow.Workflow{DontPanic: true}
	wf.Add(flow.Step(upgradeRetina))
	for _, s := range scenarioTails {
		wf.Add(flow.Step(s).DependsOn(upgradeRetina))
	}
	wf.Add(flow.Step(ensureStable).DependsOn(scenarioTails...))
	wf.Add(flow.Step(debug).DependsOn(ensureStable).When(flow.AnyFailed))

	return wf.Do(ctx)
}
