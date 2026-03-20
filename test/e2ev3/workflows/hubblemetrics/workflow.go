// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package hubblemetrics

import (
	"context"

	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/config"
	"github.com/microsoft/retina/test/e2ev3/pkg/images"
	k8s "github.com/microsoft/retina/test/e2ev3/pkg/kubernetes"
	"github.com/microsoft/retina/test/e2ev3/pkg/utils"
)

// Workflow runs the hubble metrics workflow.
type Workflow struct {
	Params *config.E2EParams
}

func (w *Workflow) String() string { return "hubble-metrics" }

func (w *Workflow) Do(ctx context.Context) error {
	p := w.Params
	kubeConfigFilePath := p.Paths.KubeConfig
	chartPath := p.Paths.HubbleChart
	imgCfg := &p.Cfg.Image
	helmCfg := &p.Cfg.Helm
	loader := images.NewLoader(*config.Provider, p.Cfg.Azure.ClusterName)

	// Construct steps.
	installHubble := &k8s.InstallHubbleHelmChart{
		Namespace:          config.KubeSystemNamespace,
		ReleaseName:        "retina",
		KubeConfigFilePath: kubeConfigFilePath,
		ChartPath:          chartPath,
		ImageTag:           imgCfg.Tag,
		ImageRegistry:      imgCfg.Registry,
		ImageNamespace:     imgCfg.Namespace,
		HelmDriver:         helmCfg.Driver,
		ImageLoader:        loader,
	}

	scenarioTails := []flow.Steper{
		addHubbleRelayValidation(kubeConfigFilePath),
		addHubbleUIValidation(kubeConfigFilePath),
	}
	for _, arch := range config.Architectures {
		scenarioTails = append(scenarioTails,
			addHubbleDNSScenario(kubeConfigFilePath, arch),
			addHubbleFlowIntraNodeScenario(kubeConfigFilePath, arch),
			addHubbleFlowInterNodeScenario(kubeConfigFilePath, arch),
			addHubbleFlowToWorldScenario(kubeConfigFilePath, arch),
			addHubbleDropScenario(kubeConfigFilePath, arch),
			addHubbleTCPScenario(kubeConfigFilePath, arch),
		)
	}

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
	wf.Add(flow.Step(installHubble))
	for _, s := range scenarioTails {
		wf.Add(flow.Step(s).DependsOn(installHubble))
	}
	wf.Add(flow.Step(ensureStable).DependsOn(scenarioTails...))
	wf.Add(flow.Step(debug).DependsOn(ensureStable).When(flow.AnyFailed))

	return wf.Do(ctx)
}
