// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package hubblemetrics

import (
	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/common"
	k8s "github.com/microsoft/retina/test/e2ev3/framework/kubernetes"
)

// InstallAndTestHubbleMetrics installs Hubble, validates its services, and runs
// DNS, flow (intra-node, inter-node, pod-to-world), drop, and TCP metric
// scenarios for each architecture.
func InstallAndTestHubbleMetrics(kubeConfigFilePath, chartPath string) *flow.Workflow {
	wf := new(flow.Workflow)

	installHubble := &k8s.InstallHubbleHelmChart{
		Namespace:          common.KubeSystemNamespace,
		ReleaseName:        "retina",
		KubeConfigFilePath: kubeConfigFilePath,
		ChartPath:          chartPath,
		TagEnv:             "TAG",
	}
	wf.Add(flow.Step(installHubble))

	var allScenarioTails []flow.Steper

	relayTail := addHubbleRelayValidation(wf, installHubble, kubeConfigFilePath)
	allScenarioTails = append(allScenarioTails, relayTail)

	uiTail := addHubbleUIValidation(wf, installHubble, kubeConfigFilePath)
	allScenarioTails = append(allScenarioTails, uiTail)

	for _, arch := range common.Architectures {
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
		PodNamespace:           common.KubeSystemNamespace,
		LabelSelector:          "k8s-app=retina",
		KubeConfigFilePath:     kubeConfigFilePath,
		IgnoreContainerRestart: false,
	}
	wf.Add(flow.Step(ensureStable).DependsOn(allScenarioTails...))

	return wf
}
