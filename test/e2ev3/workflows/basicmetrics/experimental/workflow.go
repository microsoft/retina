// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package experimental

import (
	"context"

	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/config"
	"github.com/microsoft/retina/test/e2ev3/pkg/images"
	k8s "github.com/microsoft/retina/test/e2ev3/pkg/kubernetes"
	"github.com/microsoft/retina/test/e2ev3/pkg/utils"
)

// Workflow runs the experimental basic metrics workflow.
type Workflow struct {
	Cfg *config.E2EConfig
}

func (w *Workflow) String() string { return "basic-metrics-experimental" }

func (w *Workflow) Do(ctx context.Context) error {
	p := w.Cfg
	restConfig := p.RestConfig
	chartPath := p.Paths.RetinaChart
	testPodNamespace := config.TestPodNamespace
	imgCfg := &p.Image
	helmCfg := &p.Helm
	loader := images.NewLoader(*config.Provider, p.Azure.ClusterName)

	// Construct steps.
	installRetina := &k8s.InstallHelmChart{
		Namespace:          config.KubeSystemNamespace,
		ReleaseName:        "retina",
		KubeConfigFilePath: p.Paths.KubeConfig,
		ChartPath:          chartPath,
		ImageTag:           imgCfg.Tag,
		ImageRegistry:      imgCfg.Registry,
		ImageNamespace:     imgCfg.Namespace,
		HelmDriver:         helmCfg.Driver,
		ImageLoader:        loader,
	}

	var scenarios []flow.Steper
	for _, arch := range config.Architectures {
		scenarios = append(scenarios,
			addForwardScenario(restConfig, testPodNamespace, arch),
			addConntrackScenario(restConfig, testPodNamespace, arch),
			addTCPStatsScenario(restConfig, testPodNamespace, arch),
		)
	}
	scenarios = append(scenarios,
		addNetworkStatsScenario(restConfig),
		addNodeConnectivityScenario(restConfig),
	)

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
	wf := &flow.Workflow{DontPanic: true}
	wf.Add(flow.Step(installRetina))
	for _, s := range scenarios {
		wf.Add(flow.Step(s).DependsOn(installRetina))
	}
	wf.Add(flow.Step(ensureStable).DependsOn(scenarios...))
	wf.Add(flow.Step(debug).DependsOn(ensureStable).When(flow.AnyFailed))

	return wf.Do(ctx)
}
