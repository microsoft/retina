// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package advancedmetrics

import (
	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/config"
	k8s "github.com/microsoft/retina/test/e2ev3/pkg/kubernetes"
	"github.com/microsoft/retina/test/e2ev3/pkg/utils"
)

// UpgradeAndTestRetinaAdvancedMetrics creates a workflow that upgrades Retina
// with the advanced profile and validates advanced DNS and latency metrics.
func UpgradeAndTestRetinaAdvancedMetrics(kubeConfigFilePath, chartPath, valuesFilePath, testPodNamespace string, helmCfg *config.HelmConfig) *flow.Workflow {
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
		dnsValidTail := addAdvancedDNSScenario(wf, upgradeRetina, kubeConfigFilePath, testPodNamespace, arch,
			"valid", "nslookup kubernetes.default", false,
			"kubernetes.default.svc.cluster.local.", "A", "StatefulSet",
			"1", "kubernetes.default.svc.cluster.local.", "A", "NOERROR", "10.0.0.1",
		)
		scenarioTails = append(scenarioTails, dnsValidTail)

		dnsNXTail := addAdvancedDNSScenario(wf, upgradeRetina, kubeConfigFilePath, testPodNamespace, arch,
			"nxdomain", "nslookup some.non.existent.domain.", true,
			"some.non.existent.domain.", "A", "StatefulSet",
			"0", "some.non.existent.domain.", "A", "NXDOMAIN", EmptyResponse,
		)
		scenarioTails = append(scenarioTails, dnsNXTail)
	}

	latencyTail := addLatencyScenario(wf, upgradeRetina, kubeConfigFilePath)
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
