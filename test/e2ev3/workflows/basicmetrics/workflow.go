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

// InstallAndTestRetinaBasicMetrics creates a workflow that installs Retina
// and validates basic metrics: drop, TCP, DNS, and Windows HNS for each architecture.
func InstallAndTestRetinaBasicMetrics(kubeConfigFilePath, chartPath, testPodNamespace string, imgCfg *config.ImageConfig, helmCfg *config.HelmConfig) *flow.Workflow {
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
		dropTail := addDropScenario(wf, installRetina, kubeConfigFilePath, testPodNamespace, arch)
		scenarioTails = append(scenarioTails, dropTail)

		tcpTail := addTCPScenario(wf, installRetina, kubeConfigFilePath, testPodNamespace, arch)
		scenarioTails = append(scenarioTails, tcpTail)

		dns1Tail := addBasicDNSScenario(wf, installRetina, kubeConfigFilePath, testPodNamespace, arch,
			"valid-domain", "nslookup kubernetes.default", false)
		scenarioTails = append(scenarioTails, dns1Tail)

		dns2Tail := addBasicDNSScenario(wf, installRetina, kubeConfigFilePath, testPodNamespace, arch,
			"nxdomain", "nslookup some.non.existent.domain", true)
		scenarioTails = append(scenarioTails, dns2Tail)

		winStep := &steps.ValidateHNSMetricStep{
			KubeConfigFilePath:       kubeConfigFilePath,
			RetinaDaemonSetNamespace: common.KubeSystemNamespace,
			RetinaDaemonSetName:      "retina-agent-win",
		}
		wf.Add(flow.Step(winStep).DependsOn(installRetina))
		scenarioTails = append(scenarioTails, winStep)
	}

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
