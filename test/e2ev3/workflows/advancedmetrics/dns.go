// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package advancedmetrics

import (
	"context"
	"fmt"
	"log"

	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/config"
	k8s "github.com/microsoft/retina/test/e2ev3/pkg/kubernetes"
	prom "github.com/microsoft/retina/test/e2ev3/pkg/prometheus"
	"github.com/microsoft/retina/test/e2ev3/pkg/utils"
)

func addAdvancedDNSScenario(wf *flow.Workflow, upstream flow.Steper, kubeConfigFilePath, namespace, arch, variant string,
	command string, expectError bool,
	reqQuery, reqQueryType, workloadKind string,
	respNumResponse, respQuery, respQueryType, respReturnCode, respResponse string,
) flow.Steper {
	agnhostName := "agnhost-adv-dns-" + variant + "-" + arch
	podName := agnhostName + "-0"

	createAgnhost := &k8s.CreateAgnhostStatefulSet{
		AgnhostName: agnhostName, AgnhostNamespace: namespace, AgnhostArch: arch, KubeConfigFilePath: kubeConfigFilePath,
	}
	execCmd1 := flow.Func("adv-dns-"+variant+"-1-"+arch, func(ctx context.Context) error {
		err := (&k8s.ExecInPod{PodName: podName, PodNamespace: namespace, Command: command, KubeConfigFilePath: kubeConfigFilePath}).Do(ctx)
		if expectError {
			return nil
		}
		return err
	})
	execCmd2 := flow.Func("adv-dns-"+variant+"-2-"+arch, func(ctx context.Context) error {
		err := (&k8s.ExecInPod{PodName: podName, PodNamespace: namespace, Command: command, KubeConfigFilePath: kubeConfigFilePath}).Do(ctx)
		if expectError {
			return nil
		}
		return err
	})
	validateReq := &ValidateAdvancedDNSRequestStep{
		PodNamespace: namespace, PodName: podName, Query: reqQuery, QueryType: reqQueryType,
		WorkloadKind: workloadKind, WorkloadName: agnhostName, KubeConfigFilePath: kubeConfigFilePath,
	}
	validateResp := &ValidateAdvancedDNSResponseStep{
		PodNamespace: namespace, NumResponse: respNumResponse, PodName: podName,
		Query: respQuery, QueryType: respQueryType, Response: respResponse, ReturnCode: respReturnCode,
		WorkloadKind: workloadKind, WorkloadName: agnhostName, KubeConfigFilePath: kubeConfigFilePath,
	}
	validateWithPF := &utils.WithPortForward{
		PF: &k8s.PortForward{
			Namespace: config.KubeSystemNamespace, LabelSelector: "k8s-app=retina",
			LocalPort: config.RetinaMetricsPort, RemotePort: config.RetinaMetricsPort,
			Endpoint: "metrics", KubeConfigFilePath: kubeConfigFilePath, OptionalLabelAffinity: "app=" + agnhostName,
		},
		Steps: []flow.Steper{validateReq, validateResp},
	}
	deleteAgnhost := &k8s.DeleteKubernetesResource{
		ResourceType: k8s.TypeString(k8s.StatefulSet), ResourceName: agnhostName, ResourceNamespace: namespace, KubeConfigFilePath: kubeConfigFilePath,
	}

	// Setup: provision resources and generate traffic.
	wf.Add(
		flow.Pipe(createAgnhost, execCmd1, execCmd2).
			DependsOn(upstream).
			Timeout(utils.DefaultScenarioTimeout),
	)

	// Validate: retry with exponential backoff until metrics appear.
	wf.Add(
		flow.Step(validateWithPF).
			DependsOn(execCmd2).
			Retry(utils.RetryWithBackoff),
	)

	// Cleanup: always runs, even if validation fails.
	wf.Add(
		flow.Pipe(deleteAgnhost).
			DependsOn(validateWithPF).
			When(flow.Always),
	)
	return deleteAgnhost
}



// EmptyResponse is a sentinel value that gets converted to an empty string
// for metric label matching.
const EmptyResponse = "emptyResponse"

var (
	dnsAdvRequestCountMetricName  = "networkobservability_adv_dns_request_count"
	dnsAdvResponseCountMetricName = "networkobservability_adv_dns_response_count"
)

// ValidateAdvancedDNSRequestStep checks the advanced DNS request count metric
// with labels including pod IP, namespace, pod name, query info, and workload info.
type ValidateAdvancedDNSRequestStep struct {
	PodNamespace       string
	PodName            string
	Query              string
	QueryType          string
	WorkloadKind       string
	WorkloadName       string
	KubeConfigFilePath string
}

func (v *ValidateAdvancedDNSRequestStep) Do(_ context.Context) error {
	metricsEndpoint := fmt.Sprintf("http://localhost:%s/metrics", config.RetinaMetricsPort)

	podIP, err := k8s.GetPodIP(v.KubeConfigFilePath, v.PodNamespace, v.PodName)
	if err != nil {
		return fmt.Errorf("failed to get pod IP address: %w", err)
	}

	validateAdvancedDNSRequestMetrics := map[string]string{
		"ip":            podIP,
		"namespace":     v.PodNamespace,
		"podname":       v.PodName,
		"query":         v.Query,
		"query_type":    v.QueryType,
		"workload_kind": v.WorkloadKind,
		"workload_name": v.WorkloadName,
	}

	err = prom.CheckMetric(metricsEndpoint, dnsAdvRequestCountMetricName, validateAdvancedDNSRequestMetrics)
	if err != nil {
		return fmt.Errorf("failed to verify advance dns request metrics %s: %w", dnsAdvRequestCountMetricName, err)
	}
	log.Printf("found metrics matching %+v\n", dnsAdvRequestCountMetricName)
	return nil
}

// ValidateAdvancedDNSResponseStep checks the advanced DNS response count metric
// with labels including pod IP, namespace, pod name, num_response, query info,
// response, return_code, and workload info.
type ValidateAdvancedDNSResponseStep struct {
	PodNamespace       string
	NumResponse        string
	PodName            string
	Query              string
	QueryType          string
	Response           string
	ReturnCode         string
	WorkloadKind       string
	WorkloadName       string
	KubeConfigFilePath string
}

func (v *ValidateAdvancedDNSResponseStep) Do(_ context.Context) error {
	metricsEndpoint := fmt.Sprintf("http://localhost:%s/metrics", config.RetinaMetricsPort)

	podIP, err := k8s.GetPodIP(v.KubeConfigFilePath, v.PodNamespace, v.PodName)
	if err != nil {
		return fmt.Errorf("failed to get pod IP address: %w", err)
	}

	if v.Response == EmptyResponse {
		v.Response = ""
	}

	validateAdvanceDNSResponseMetrics := map[string]string{
		"ip":            podIP,
		"namespace":     v.PodNamespace,
		"num_response":  v.NumResponse,
		"podname":       v.PodName,
		"query":         v.Query,
		"query_type":    v.QueryType,
		"response":      v.Response,
		"return_code":   v.ReturnCode,
		"workload_kind": v.WorkloadKind,
		"workload_name": v.WorkloadName,
	}

	err = prom.CheckMetric(metricsEndpoint, dnsAdvResponseCountMetricName, validateAdvanceDNSResponseMetrics)
	if err != nil {
		return fmt.Errorf("failed to verify advance dns response metrics %s: %w", dnsAdvResponseCountMetricName, err)
	}
	log.Printf("found metrics matching %+v\n", dnsAdvResponseCountMetricName)
	return nil
}
