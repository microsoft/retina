// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package hubblemetrics

import (
	"context"

	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/config"
	k8s "github.com/microsoft/retina/test/e2ev3/pkg/kubernetes"
	"github.com/microsoft/retina/test/e2ev3/pkg/utils"
)

// Workflow runs the hubble metrics workflow.
type Workflow struct {
	Cfg *config.E2EConfig
}

func (w *Workflow) String() string { return "hubble-metrics" }

func (w *Workflow) Do(ctx context.Context) error {
	p := w.Cfg
	restConfig := p.Cluster.RestConfig()
	chartPath := p.Paths.HubbleChart
	imgCfg := &p.Image
	helmCfg := &p.Helm

	// Construct steps.
	installHubble := &k8s.InstallHubbleHelmChart{
		Namespace:          config.KubeSystemNamespace,
		ReleaseName:        "retina",
		KubeConfigFilePath: p.Cluster.KubeConfigPath(),
		ChartPath:          chartPath,
		ImageTag:           imgCfg.Tag,
		ImageRegistry:      imgCfg.Registry,
		ImageNamespace:     imgCfg.Namespace,
		HelmDriver:         helmCfg.Driver,
		ImageLoader:        p.Cluster,
	}

	scenarios := []flow.Steper{
		addHubbleRelayValidation(restConfig),
		addHubbleUIValidation(restConfig),
	}
	for _, arch := range config.Architectures {
		scenarios = append(scenarios,
			addHubbleDNSScenario(restConfig, arch),
			addHubbleFlowIntraNodeScenario(restConfig, arch),
			addHubbleFlowInterNodeScenario(restConfig, arch),
			addHubbleFlowToWorldScenario(restConfig, arch),
			addHubbleDropScenario(restConfig, arch),
			addHubbleTCPScenario(restConfig, arch),
		)
	}

	ensureStable := &k8s.EnsureStableComponent{
		PodNamespace:           config.KubeSystemNamespace,
		LabelSelector:          "k8s-app=retina",
		RestConfig:             restConfig,
		IgnoreContainerRestart: false,
	}

	debug := &utils.DebugOnFailure{
		RestConfig:    restConfig,
		Namespace:     config.KubeSystemNamespace,
		LabelSelector: "k8s-app=retina",
	}

	// Wire dependencies and register.
	// Scenarios run sequentially because they share the same port-forward port.
	wf := &flow.Workflow{DontPanic: true}
	wf.Add(flow.Step(installHubble))
	prev := flow.Steper(installHubble)
	for _, s := range scenarios {
		wf.Add(flow.Step(s).DependsOn(prev))
		prev = s
	}
	wf.Add(flow.Step(ensureStable).DependsOn(prev))
	wf.Add(flow.Step(debug).DependsOn(ensureStable).When(flow.AnyFailed))

	return wf.Do(ctx)
}
