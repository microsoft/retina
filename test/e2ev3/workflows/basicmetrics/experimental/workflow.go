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

// Workflow runs the experimental basic metrics workflow.
type Workflow struct {
	Cfg *config.E2EConfig
}

func (w *Workflow) String() string { return "basic-metrics-experimental" }

func (w *Workflow) Do(ctx context.Context) error {
	ctx, _ = utils.StepLogger(ctx, w)
	p := w.Cfg
	restConfig := p.Cluster.RestConfig()
	chartPath := p.Paths.RetinaChart
	testPodNamespace := "basic-metrics-exp-test"
	imgCfg := &p.Image
	helmCfg := &p.Helm

	// Construct steps.
	installRetina := &k8s.InstallHelmChart{
		Namespace:          config.KubeSystemNamespace,
		ReleaseName:        "retina",
		KubeConfigFilePath: p.Cluster.KubeConfigPath(),
		ChartPath:          chartPath,
		ImageTag:           imgCfg.Tag,
		ImageRegistry:      imgCfg.Registry,
		ImageNamespace:     imgCfg.Namespace,
		HelmDriver:         helmCfg.Driver,
		ImageLoader:        p.Cluster,
		ValuesFile:         p.Paths.ExperimentalProfile,
		TestPodNamespace:   testPodNamespace,
	}


	var scenarios []flow.Steper
	for _, arch := range config.Architectures {
		scenarios = append(scenarios,
			addForwardScenario(restConfig, testPodNamespace, arch),
			addConntrackScenario(restConfig, testPodNamespace, arch),
			addTCPStatsScenario(restConfig, testPodNamespace, arch),
		)
	}
	// network_stats and node_connectivity metrics rely on host-level counters
	// that are typically zero or unavailable on Kind nodes, so skip them.
	if *config.Provider == "kind" {
		wfName := w.String()
		if p.Summary != nil {
			p.Summary.Skip(wfName, "network_stats", "host-level counters are zero on Kind nodes")
			p.Summary.Skip(wfName, "node_connectivity", "host-level counters are zero on Kind nodes")
		}
	} else {
		for _, arch := range config.Architectures {
			scenarios = append(scenarios,
				addNetworkStatsScenario(restConfig, testPodNamespace, arch),
				addNodeConnectivityScenario(restConfig, testPodNamespace, arch),
			)
		}
	}

	ensureStable := &k8s.EnsureStableComponent{
		PodNamespace:           config.KubeSystemNamespace,
		LabelSelector:          "k8s-app=retina",
		RestConfig:             restConfig,
		IgnoreContainerRestart: false,
	}

	debug := &k8s.DebugOnFailure{
		RestConfig: restConfig,
		Namespace:          config.KubeSystemNamespace,
		LabelSelector:      "k8s-app=retina",
	}

	// Wire dependencies and register.
	// Scenarios run sequentially because they share the same port-forward port.
	wf := &flow.Workflow{DontPanic: true}
	wf.Add(flow.Step(installRetina))
	prev := flow.Steper(installRetina)
	for _, s := range scenarios {
		wf.Add(flow.Step(s).DependsOn(prev))
		prev = s
	}
	wf.Add(flow.Step(ensureStable).DependsOn(prev))
	wf.Add(flow.Step(debug).DependsOn(ensureStable).When(flow.AnyFailed))

	return wf.Do(ctx)
}
