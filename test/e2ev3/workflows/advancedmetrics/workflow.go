// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package advancedmetrics

import (
	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/common"
	"github.com/microsoft/retina/test/e2ev3/framework/generic"
	k8s "github.com/microsoft/retina/test/e2ev3/framework/kubernetes"
	"github.com/microsoft/retina/test/e2ev3/steps"
)

// UpgradeAndTestRetinaAdvancedMetrics creates a workflow that upgrades Retina
// with the advanced profile and validates advanced DNS and latency metrics.
func UpgradeAndTestRetinaAdvancedMetrics(kubeConfigFilePath, chartPath, valuesFilePath, testPodNamespace string) *flow.Workflow {
	wf := new(flow.Workflow)

	upgradeRetina := &k8s.UpgradeRetinaHelmChart{
		Namespace:          common.KubeSystemNamespace,
		ReleaseName:        "retina",
		KubeConfigFilePath: kubeConfigFilePath,
		ChartPath:          chartPath,
		TagEnv:             generic.DefaultTagEnv,
		ValuesFile:         valuesFilePath,
	}
	wf.Add(flow.Step(upgradeRetina))

	var scenarioTails []flow.Steper

	for _, arch := range common.Architectures {
		dnsValidTail := addAdvancedDNSScenario(wf, upgradeRetina, kubeConfigFilePath, testPodNamespace, arch,
			"valid", "nslookup kubernetes.default", false,
			"kubernetes.default.svc.cluster.local.", "A", "StatefulSet",
			"1", "kubernetes.default.svc.cluster.local.", "A", "NOERROR", "10.0.0.1",
		)
		scenarioTails = append(scenarioTails, dnsValidTail)

		dnsNXTail := addAdvancedDNSScenario(wf, upgradeRetina, kubeConfigFilePath, testPodNamespace, arch,
			"nxdomain", "nslookup some.non.existent.domain.", true,
			"some.non.existent.domain.", "A", "StatefulSet",
			"0", "some.non.existent.domain.", "A", "NXDOMAIN", steps.EmptyResponse,
		)
		scenarioTails = append(scenarioTails, dnsNXTail)
	}

	latencyTail := addLatencyScenario(wf, upgradeRetina, kubeConfigFilePath)
	scenarioTails = append(scenarioTails, latencyTail)

	ensureStable := &k8s.EnsureStableComponent{
		PodNamespace:           common.KubeSystemNamespace,
		LabelSelector:          "k8s-app=retina",
		KubeConfigFilePath:     kubeConfigFilePath,
		IgnoreContainerRestart: false,
	}
	wf.Add(flow.Step(ensureStable).DependsOn(scenarioTails...))

	return wf
}
