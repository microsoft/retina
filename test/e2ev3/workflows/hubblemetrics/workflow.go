// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package hubblemetrics

import (
	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/config"
	"github.com/microsoft/retina/test/e2ev3/pkg/images"
	k8s "github.com/microsoft/retina/test/e2ev3/pkg/kubernetes"
	"github.com/microsoft/retina/test/e2ev3/pkg/utils"
)

// InstallAndTestHubbleMetrics installs Hubble, validates its services, and runs
// DNS, flow (intra-node, inter-node, pod-to-world), drop, and TCP metric
// scenarios for each architecture.
func InstallAndTestHubbleMetrics(kubeConfigFilePath, chartPath string, imgCfg *config.ImageConfig, helmCfg *config.HelmConfig, loader images.Loader) *flow.Workflow {
	wf := &flow.Workflow{DontPanic: true}

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
	wf.Add(flow.Step(installHubble))

	var allScenarioTails []flow.Steper

	relayTail := addHubbleRelayValidation(wf, installHubble, kubeConfigFilePath)
	allScenarioTails = append(allScenarioTails, relayTail)

	uiTail := addHubbleUIValidation(wf, installHubble, kubeConfigFilePath)
	allScenarioTails = append(allScenarioTails, uiTail)

	for _, arch := range config.Architectures {
		dnsTail := addHubbleDNSScenario(wf, installHubble, kubeConfigFilePath, arch)
		allScenarioTails = append(allScenarioTails, dnsTail)

		flowIntraTail := addHubbleFlowIntraNodeScenario(wf, installHubble, kubeConfigFilePath, arch)
		allScenarioTails = append(allScenarioTails, flowIntraTail)

		flowInterTail := addHubbleFlowInterNodeScenario(wf, installHubble, kubeConfigFilePath, arch)
		allScenarioTails = append(allScenarioTails, flowInterTail)

		flowWorldTail := addHubbleFlowToWorldScenario(wf, installHubble, kubeConfigFilePath, arch)
		allScenarioTails = append(allScenarioTails, flowWorldTail)

		dropTail := addHubbleDropScenario(wf, installHubble, kubeConfigFilePath, arch)
		allScenarioTails = append(allScenarioTails, dropTail)

		tcpTail := addHubbleTCPScenario(wf, installHubble, kubeConfigFilePath, arch)
		allScenarioTails = append(allScenarioTails, tcpTail)
	}

	ensureStable := &k8s.EnsureStableComponent{
		PodNamespace:           config.KubeSystemNamespace,
		LabelSelector:          "k8s-app=retina",
		KubeConfigFilePath:     kubeConfigFilePath,
		IgnoreContainerRestart: false,
	}
	wf.Add(flow.Step(ensureStable).DependsOn(allScenarioTails...))

	debug := &utils.DebugOnFailure{
		KubeConfigFilePath: kubeConfigFilePath,
		Namespace:          config.KubeSystemNamespace,
		LabelSelector:      "k8s-app=retina",
	}
	wf.Add(flow.Step(debug).DependsOn(ensureStable).When(flow.AnyFailed))

	return wf
}
