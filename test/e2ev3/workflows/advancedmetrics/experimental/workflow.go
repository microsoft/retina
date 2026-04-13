// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package experimental

import (
	"context"
	"log/slog"

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
	ctx, _ = utils.StepLogger(ctx, w)
	p := w.Cfg
	restConfig := p.Cluster.RestConfig()
	chartPath := p.Paths.RetinaChart
	valuesFilePath := p.Paths.AdvancedProfile
	testPodNamespace := "advanced-metrics-exp-test"
	helmCfg := &p.Helm

	// Construct steps.
	createNS := &k8s.CreateNamespace{
		Namespace:  testPodNamespace,
		RestConfig: restConfig,
	}
	upgradeRetina := &k8s.UpgradeRetinaHelmChart{
		Namespace:          config.KubeSystemNamespace,
		ReleaseName:        "retina",
		KubeConfigFilePath: p.Cluster.KubeConfigPath(),
		ChartPath:          chartPath,
		HelmDriver:         helmCfg.Driver,
		ValuesFile:         valuesFilePath,
	}


	isKind := *config.Provider == "kind"
	wfName := w.String()

	var scenarios []flow.Steper
	for _, arch := range config.Architectures {
		// Drop scenario requires NetworkPolicy enforcement via dropreason eBPF
		// hooks which do not capture drops on Kind.
		if isKind {
			reason := "dropreason eBPF hooks do not capture drops on Kind"
			slog.Info("SKIP: adv_drop_count/bytes — " + reason)
			if p.Summary != nil {
				p.Summary.Skip(wfName, "adv_drop_count/bytes", reason)
			}
		} else {
			scenarios = append(scenarios, addAdvancedDropScenario(restConfig, testPodNamespace, arch))
		}

		scenarios = append(scenarios,
			addAdvancedForwardScenario(restConfig, testPodNamespace, arch),
			addAdvancedTCPScenario(restConfig, testPodNamespace, arch, isKind, p),
		)
	}
	scenarios = append(scenarios, addAPIServerLatencyScenario(restConfig))

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
	wf.Add(flow.Step(createNS))
	wf.Add(flow.Step(upgradeRetina).DependsOn(createNS))
	prev := flow.Steper(upgradeRetina)
	for _, s := range scenarios {
		wf.Add(flow.Step(s).DependsOn(prev))
		prev = s
	}
	wf.Add(flow.Step(ensureStable).DependsOn(prev))
	wf.Add(flow.Step(debug).DependsOn(ensureStable).When(flow.AnyFailed))

	return wf.Do(ctx)
}
