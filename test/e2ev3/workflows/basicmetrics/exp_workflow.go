// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package basicmetrics

import (
	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/common"
	"github.com/microsoft/retina/test/e2ev3/pkg/config"
	k8s "github.com/microsoft/retina/test/e2ev3/pkg/kubernetes"
	"github.com/microsoft/retina/test/e2ev3/steps"
)

// InstallAndTestRetinaBasicMetricsExperimental creates a workflow that installs Retina
// and validates experimental basic metrics: forward, conntrack, TCP stats,
// network stats (IP/UDP/interface), and node connectivity.
func InstallAndTestRetinaBasicMetricsExperimental(kubeConfigFilePath, chartPath, testPodNamespace string, imgCfg *config.ImageConfig, helmCfg *config.HelmConfig) *flow.Workflow {
	wf := &flow.Workflow{DontPanic: true}

	installRetina := &k8s.InstallHelmChart{
		Namespace:          common.KubeSystemNamespace,
		ReleaseName:        "retina",
		KubeConfigFilePath: kubeConfigFilePath,
		ChartPath:          chartPath,
		ImageTag:           imgCfg.Tag,
		ImageRegistry:      imgCfg.Registry,
		ImageNamespace:     imgCfg.Namespace,
		HelmDriver:         helmCfg.Driver,
	}
	wf.Add(flow.Step(installRetina))

	var scenarioTails []flow.Steper

	for _, arch := range common.Architectures {
		fwdTail := addForwardScenario(wf, installRetina, kubeConfigFilePath, testPodNamespace, arch)
		scenarioTails = append(scenarioTails, fwdTail)

		ctTail := addConntrackScenario(wf, installRetina, kubeConfigFilePath, testPodNamespace, arch)
		scenarioTails = append(scenarioTails, ctTail)

		tcpStatsTail := addTCPStatsScenario(wf, installRetina, kubeConfigFilePath, testPodNamespace, arch)
		scenarioTails = append(scenarioTails, tcpStatsTail)
	}

	// Node-level metrics — not per-arch.
	netStatsTail := addNetworkStatsScenario(wf, installRetina, kubeConfigFilePath)
	scenarioTails = append(scenarioTails, netStatsTail)

	nodeConnTail := addNodeConnectivityScenario(wf, installRetina, kubeConfigFilePath)
	scenarioTails = append(scenarioTails, nodeConnTail)

	ensureStable := &k8s.EnsureStableComponent{
		PodNamespace:           common.KubeSystemNamespace,
		LabelSelector:          "k8s-app=retina",
		KubeConfigFilePath:     kubeConfigFilePath,
		IgnoreContainerRestart: false,
	}
	wf.Add(flow.Step(ensureStable).DependsOn(scenarioTails...))

	debug := &steps.DebugOnFailure{
		KubeConfigFilePath: kubeConfigFilePath,
		Namespace:          common.KubeSystemNamespace,
		LabelSelector:      "k8s-app=retina",
	}
	wf.Add(flow.Step(debug).DependsOn(ensureStable).When(flow.AnyFailed))

	return wf
}
