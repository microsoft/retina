// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package dns

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/microsoft/retina/test/e2e/common"
	"github.com/microsoft/retina/test/e2e/framework/constants"
	"github.com/microsoft/retina/test/e2e/framework/kubernetes"
	"github.com/microsoft/retina/test/e2e/framework/types"
)

const (
	sleepDelay    = 5 * time.Second
	EmptyResponse = "emptyResponse"
)

type RequestValidationParams struct {
	NumResponse string
	Query       string
	QueryType   string

	Command     string
	ExpectError bool
}

type ResponseValidationParams struct {
	NumResponse string
	Query       string
	QueryType   string
	ReturnCode  string
	Response    string
}

// ValidateBasicDNSMetrics validates basic DNS metrics present in the metrics endpoint
func ValidateBasicDNSMetrics(scenarioName string, req *RequestValidationParams, resp *ResponseValidationParams, namespace, arch string) *types.Scenario {
	// generate a random ID using rand
	id := fmt.Sprintf("basic-dns-port-forward-%d", rand.Int()) // nolint:gosec // fine to use math/rand here
	agnhostName := "agnhost-" + id
	podName := agnhostName + "-0"
	steps := []*types.StepWrapper{
		{
			Step: &kubernetes.CreateAgnhostStatefulSet{
				AgnhostName:      agnhostName,
				AgnhostNamespace: namespace,
				AgnhostArch:      arch,
			},
		},
		// Need this delay to guarantee that the pods will have bpf program attached
		{
			Step: &types.Sleep{
				Duration: 30 * time.Second,
			},
			Opts: &types.StepOptions{
				SkipSavingParametersToJob: true,
			},
		},
		{
			Step: &kubernetes.ExecInPod{
				PodName:      podName,
				PodNamespace: namespace,
				Command:      req.Command,
			},
			Opts: &types.StepOptions{
				ExpectError:               req.ExpectError,
				SkipSavingParametersToJob: true,
			},
		},
		{
			Step: &types.Sleep{
				Duration: sleepDelay,
			},
		},
		// Ref: https://github.com/microsoft/retina/issues/415
		{
			Step: &kubernetes.ExecInPod{
				PodName:      podName,
				PodNamespace: namespace,
				Command:      req.Command,
			},
			Opts: &types.StepOptions{
				ExpectError:               req.ExpectError,
				SkipSavingParametersToJob: true,
			},
		},
		{
			Step: &types.Sleep{
				Duration: sleepDelay,
			},
		},
		{
			Step: &kubernetes.PortForward{
				Namespace:             common.KubeSystemNamespace,
				LabelSelector:         "k8s-app=retina",
				LocalPort:             constants.RetinaMetricsPort,
				RemotePort:            constants.RetinaMetricsPort,
				Endpoint:              "metrics",
				OptionalLabelAffinity: "app=" + agnhostName, // port forward to a pod on a node that also has this pod with this label, assuming same namespace
			},
			Opts: &types.StepOptions{
				SkipSavingParametersToJob: true,
				RunInBackgroundWithID:     id,
			},
		},
		{
			Step: &validateBasicDNSRequestMetrics{
				Query:     req.Query,
				QueryType: req.QueryType,
			},
			Opts: &types.StepOptions{
				SkipSavingParametersToJob: true,
			},
		},
		{
			Step: &validateBasicDNSResponseMetrics{
				NumResponse: resp.NumResponse,
				Query:       resp.Query,
				QueryType:   resp.QueryType,
				ReturnCode:  resp.ReturnCode,
				Response:    resp.Response,
			},
			Opts: &types.StepOptions{
				SkipSavingParametersToJob: true,
			},
		},
		{
			Step: &types.Stop{
				BackgroundID: id,
			},
		},
		{
			Step: &kubernetes.DeleteKubernetesResource{
				ResourceType:      kubernetes.TypeString(kubernetes.StatefulSet),
				ResourceName:      agnhostName,
				ResourceNamespace: namespace,
			}, Opts: &types.StepOptions{
				SkipSavingParametersToJob: true,
			},
		},
		{
			Step: &types.Sleep{
				Duration: sleepDelay,
			},
		},
	}
	return types.NewScenario(scenarioName, steps...)
}

// ValidateAdvancedDNSMetrics validates the advanced DNS metrics present in the metrics endpoint
func ValidateAdvancedDNSMetrics(scenarioName string, req *RequestValidationParams, resp *ResponseValidationParams, kubeConfigFilePath, namespace, arch string) *types.Scenario {
	// random ID
	id := fmt.Sprintf("adv-dns-port-forward-%d", rand.Int()) // nolint:gosec // fine to use math/rand here
	agnhostName := "agnhost-" + id
	podName := agnhostName + "-0"
	steps := []*types.StepWrapper{
		{
			Step: &kubernetes.CreateAgnhostStatefulSet{
				AgnhostName:      agnhostName,
				AgnhostNamespace: namespace,
				AgnhostArch:      arch,
			},
		},
		// Need this delay to guarantee that the pods will have bpf program attached
		{
			Step: &types.Sleep{
				Duration: 30 * time.Second,
			},
			Opts: &types.StepOptions{
				SkipSavingParametersToJob: true,
			},
		},
		{
			Step: &kubernetes.ExecInPod{
				PodName:      podName,
				PodNamespace: namespace,
				Command:      req.Command,
			},
			Opts: &types.StepOptions{
				ExpectError:               req.ExpectError,
				SkipSavingParametersToJob: true,
			},
		},
		{
			Step: &types.Sleep{
				Duration: sleepDelay,
			},
		},
		// Ref: https://github.com/microsoft/retina/issues/415
		{
			Step: &kubernetes.ExecInPod{
				PodName:      podName,
				PodNamespace: namespace,
				Command:      req.Command,
			},
			Opts: &types.StepOptions{
				ExpectError:               req.ExpectError,
				SkipSavingParametersToJob: true,
			},
		},
		{
			Step: &types.Sleep{
				Duration: sleepDelay,
			},
		},
		{
			Step: &kubernetes.PortForward{
				Namespace:             common.KubeSystemNamespace,
				LabelSelector:         "k8s-app=retina",
				LocalPort:             constants.RetinaMetricsPort,
				RemotePort:            constants.RetinaMetricsPort,
				Endpoint:              "metrics",
				OptionalLabelAffinity: "app=" + agnhostName, // port forward to a pod on a node that also has this pod with this label, assuming same namespace
			},
			Opts: &types.StepOptions{
				SkipSavingParametersToJob: true,
				RunInBackgroundWithID:     id,
			},
		},
		{
			Step: &ValidateAdvancedDNSRequestMetrics{
				PodNamespace:       namespace,
				PodName:            podName,
				Query:              req.Query,
				QueryType:          req.QueryType,
				WorkloadKind:       "StatefulSet",
				WorkloadName:       agnhostName,
				KubeConfigFilePath: kubeConfigFilePath,
			},
			Opts: &types.StepOptions{
				SkipSavingParametersToJob: true,
			},
		},
		{
			Step: &ValidateAdvanceDNSResponseMetrics{
				PodNamespace:       namespace,
				NumResponse:        resp.NumResponse,
				PodName:            podName,
				Query:              resp.Query,
				QueryType:          resp.QueryType,
				Response:           resp.Response,
				ReturnCode:         resp.ReturnCode,
				WorkloadKind:       "StatefulSet",
				WorkloadName:       agnhostName,
				KubeConfigFilePath: kubeConfigFilePath,
			},
			Opts: &types.StepOptions{
				SkipSavingParametersToJob: true,
			},
		},
		{
			Step: &types.Stop{
				BackgroundID: id,
			},
		},
		{
			Step: &kubernetes.DeleteKubernetesResource{
				ResourceType:      kubernetes.TypeString(kubernetes.StatefulSet),
				ResourceName:      agnhostName,
				ResourceNamespace: namespace,
			}, Opts: &types.StepOptions{
				SkipSavingParametersToJob: true,
			},
		},
		{
			Step: &types.Sleep{
				Duration: sleepDelay,
			},
		},
	}
	return types.NewScenario(scenarioName, steps...)
}
