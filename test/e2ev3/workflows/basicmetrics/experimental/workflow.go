// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package experimental

import (
	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/config"
	"github.com/microsoft/retina/test/e2ev3/pkg/images"
	k8s "github.com/microsoft/retina/test/e2ev3/pkg/kubernetes"
	"github.com/microsoft/retina/test/e2ev3/pkg/utils"
)

// InstallAndTestRetinaBasicMetricsExperimental creates a workflow that installs Retina
// and validates experimental basic metrics: forward, conntrack, TCP stats,
// network stats (IP/UDP/interface), and node connectivity.
func InstallAndTestRetinaBasicMetricsExperimental(kubeConfigFilePath, chartPath, testPodNamespace string, imgCfg *config.ImageConfig, helmCfg *config.HelmConfig, loader images.Loader) *flow.Workflow {
	wf := &flow.Workflow{DontPanic: true}

	installRetina := &k8s.InstallHelmChart{
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
	wf.Add(flow.Step(installRetina))

	var scenarioTails []flow.Steper

	for _, arch := range config.Architectures {
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
