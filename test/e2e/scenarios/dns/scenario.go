// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package dns

import (
	"strconv"
	"time"

	"github.com/microsoft/retina/test/e2e/common"
	"github.com/microsoft/retina/test/e2e/framework/kubernetes"
	"github.com/microsoft/retina/test/e2e/framework/types"
)

const sleepDelay = 5 * time.Second

// ValidateBasicDNSMetrics validates basic DNS metrics
func ValidateBasicDNSMetrics() *types.Scenario {
	name := "Validate Basic DNS Metrics"
	steps := []*types.StepWrapper{
		{
			Step: &kubernetes.CreateAgnhostStatefulSet{
				AgnhostName:      "agnhost-a",
				AgnhostNamespace: "kube-system",
			},
		},
		{
			Step: &kubernetes.ExecInPod{
				PodName:      "agnhost-a-0",
				PodNamespace: "kube-system",
				Command:      "nslookup kubernetes.default",
			},
			Opts: &types.StepOptions{
				ExpectError:               false,
				SkipSavingParamatersToJob: true,
			},
		},
		{
			Step: &types.Sleep{
				Duration: sleepDelay,
			},
		},
		{
			Step: &kubernetes.PortForward{
				Namespace:             "kube-system",
				LabelSelector:         "k8s-app=retina",
				LocalPort:             strconv.Itoa(common.RetinaPort),
				RemotePort:            strconv.Itoa(common.RetinaPort),
				OptionalLabelAffinity: "app=agnhost-a", // port forward to a pod on a node that also has this pod with this label, assuming same namespace
			},
			Opts: &types.StepOptions{
				RunInBackgroundWithID: "dns-port-forward",
			},
		},
		{
			Step: &validateBasicDNSRequestMetrics{
				NumResponse: "0",
				Query:       "kubernetes.default.svc.cluster.local.",
				QueryType:   "A",
			},
			Opts: &types.StepOptions{
				SkipSavingParamatersToJob: true,
			},
		},
		{
			Step: &ValidateBasicDNSResponseMetrics{
				NumResponse: "1",
				Query:       "kubernetes.default.svc.cluster.local.",
				QueryType:   "A",
				ReturnCode:  "No Error",
				Response:    "10.0.0.1",
			},
			Opts: &types.StepOptions{
				SkipSavingParamatersToJob: true,
			},
		},
		{
			Step: &types.Stop{
				BackgroundID: "dns-port-forward",
			},
		},
	}
	return types.NewScenario(name, steps...)
}

// ValidateAdvanceDNSMetrics validates the advanced DNS metrics
func ValidateAdvanceDNSMetrics(kubeConfigFilePath string) *types.Scenario {
	name := "Validate Advance DNS Metrics"
	steps := []*types.StepWrapper{
		{
			Step: &kubernetes.CreateAgnhostStatefulSet{
				AgnhostName:      "agnhost-b",
				AgnhostNamespace: "kube-system",
			},
		},
		{
			Step: &kubernetes.ExecInPod{
				PodName:      "agnhost-b-0",
				PodNamespace: "kube-system",
				Command:      "nslookup kubernetes.default",
			},
			Opts: &types.StepOptions{
				ExpectError:               false,
				SkipSavingParamatersToJob: true,
			},
		},
		{
			Step: &types.Sleep{
				Duration: sleepDelay,
			},
		},
		{
			Step: &kubernetes.PortForward{
				Namespace:             "kube-system",
				LabelSelector:         "k8s-app=retina",
				LocalPort:             strconv.Itoa(common.RetinaPort),
				RemotePort:            strconv.Itoa(common.RetinaPort),
				OptionalLabelAffinity: "app=agnhost-b", // port forward to a pod on a node that also has this pod with this label, assuming same namespace
			},
			Opts: &types.StepOptions{
				RunInBackgroundWithID: "dns-port-forward",
			},
		},
		{
			Step: &ValidateAdvanceDNSRequestMetrics{
				Namespace:          "kube-system",
				NumResponse:        "0",
				PodName:            "agnhost-b-0",
				Query:              "kubernetes.default.svc.cluster.local.",
				QueryType:          "A",
				WorkloadKind:       "StatefulSet",
				WorkloadName:       "agnhost-b",
				KubeConfigFilePath: kubeConfigFilePath,
			},
			Opts: &types.StepOptions{
				SkipSavingParamatersToJob: true,
			},
		},
		{
			Step: &ValidateAdvanceDNSResponseMetrics{
				Namespace:          "kube-system",
				NumResponse:        "1",
				PodName:            "agnhost-b-0",
				Query:              "kubernetes.default.svc.cluster.local.",
				QueryType:          "A",
				Response:           "10.0.0.1",
				ReturnCode:         "NOERROR",
				WorkloadKind:       "StatefulSet",
				WorkloadName:       "agnhost-b",
				KubeConfigFilePath: kubeConfigFilePath,
			},
			Opts: &types.StepOptions{
				SkipSavingParamatersToJob: true,
			},
		},
		{
			Step: &types.Stop{
				BackgroundID: "dns-port-forward",
			},
		},
	}
	return types.NewScenario(name, steps...)
}
