// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package advancedmetrics

import (
	"context"

	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/config"
	k8s "github.com/microsoft/retina/test/e2ev3/pkg/kubernetes"
	"github.com/microsoft/retina/test/e2ev3/pkg/utils"
)

// Workflow runs the advanced metrics workflow.
type Workflow struct {
	Cfg *config.E2EConfig
}

func (w *Workflow) String() string { return "advanced-metrics" }

func (w *Workflow) Do(ctx context.Context) error {
	p := w.Cfg
	restConfig := p.RestConfig
	chartPath := p.Paths.RetinaChart
	valuesFilePath := p.Paths.AdvancedProfile
	testPodNamespace := config.TestPodNamespace
	helmCfg := &p.Helm

	// Construct steps.
	upgradeRetina := &k8s.UpgradeRetinaHelmChart{
		Namespace:          config.KubeSystemNamespace,
		ReleaseName:        "retina",
		KubeConfigFilePath: p.Paths.KubeConfig,
		ChartPath:          chartPath,
		HelmDriver:         helmCfg.Driver,
		ValuesFile:         valuesFilePath,
	}

	var scenarios []flow.Steper
	for _, arch := range config.Architectures {
		scenarios = append(scenarios,
			addAdvancedDNSScenario(restConfig, testPodNamespace, arch,
				"valid", "nslookup kubernetes.default", false,
				"kubernetes.default.svc.cluster.local.", "A", "StatefulSet",
				"1", "kubernetes.default.svc.cluster.local.", "A", "NOERROR", "10.0.0.1",
			),
			addAdvancedDNSScenario(restConfig, testPodNamespace, arch,
				"nxdomain", "nslookup some.non.existent.domain.", true,
				"some.non.existent.domain.", "A", "StatefulSet",
				"0", "some.non.existent.domain.", "A", "NXDOMAIN", EmptyResponse,
			),
		)
	}
	scenarios = append(scenarios, addLatencyScenario(restConfig))

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
	wf.Add(flow.Step(upgradeRetina))
	for _, s := range scenarios {
		wf.Add(flow.Step(s).DependsOn(upgradeRetina))
	}
	wf.Add(flow.Step(ensureStable).DependsOn(scenarios...))
	wf.Add(flow.Step(debug).DependsOn(ensureStable).When(flow.AnyFailed))

	return wf.Do(ctx)
}
