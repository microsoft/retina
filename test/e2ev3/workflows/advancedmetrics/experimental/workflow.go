// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package experimental

import (
	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/config"
	k8s "github.com/microsoft/retina/test/e2ev3/pkg/kubernetes"
	"github.com/microsoft/retina/test/e2ev3/pkg/utils"
)

// UpgradeAndTestRetinaAdvancedMetricsExperimental creates a workflow that upgrades Retina
// with the advanced profile and validates experimental advanced metrics: drop, forward,
// TCP flags/retransmissions, and API server latency.
func UpgradeAndTestRetinaAdvancedMetricsExperimental(kubeConfigFilePath, chartPath, valuesFilePath, testPodNamespace string, helmCfg *config.HelmConfig) *flow.Workflow {
	wf := &flow.Workflow{DontPanic: true}

	upgradeRetina := &k8s.UpgradeRetinaHelmChart{
		Namespace:          config.KubeSystemNamespace,
		ReleaseName:        "retina",
		KubeConfigFilePath: kubeConfigFilePath,
		ChartPath:          chartPath,
		HelmDriver:         helmCfg.Driver,
		ValuesFile:         valuesFilePath,
	}
	wf.Add(flow.Step(upgradeRetina))

	var scenarioTails []flow.Steper

	for _, arch := range config.Architectures {
		dropTail := addAdvancedDropScenario(wf, upgradeRetina, kubeConfigFilePath, testPodNamespace, arch)
		scenarioTails = append(scenarioTails, dropTail)

		fwdTail := addAdvancedForwardScenario(wf, upgradeRetina, kubeConfigFilePath, testPodNamespace, arch)
		scenarioTails = append(scenarioTails, fwdTail)

		tcpTail := addAdvancedTCPScenario(wf, upgradeRetina, kubeConfigFilePath, testPodNamespace, arch)
		scenarioTails = append(scenarioTails, tcpTail)
	}

	// API server latency is node-level, not per-arch.
	latencyTail := addAPIServerLatencyScenario(wf, upgradeRetina, kubeConfigFilePath)
	scenarioTails = append(scenarioTails, latencyTail)

	ensureStable := &k8s.EnsureStableComponent{
		PodNamespace:           config.KubeSystemNamespace,
		LabelSelector:          "k8s-app=retina",
		KubeConfigFilePath:     kubeConfigFilePath,
		IgnoreContainerRestart: false,
	}
	wf.Add(flow.Step(ensureStable).DependsOn(scenarioTails...))

	debug := &utils.DebugOnFailure{
		KubeConfigFilePath: kubeConfigFilePath,
		Namespace:          config.KubeSystemNamespace,
		LabelSelector:      "k8s-app=retina",
	}
	wf.Add(flow.Step(debug).DependsOn(ensureStable).When(flow.AnyFailed))

	return wf
}
