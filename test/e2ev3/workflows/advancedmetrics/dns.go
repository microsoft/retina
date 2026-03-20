// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package advancedmetrics

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/config"
	k8s "github.com/microsoft/retina/test/e2ev3/pkg/kubernetes"
	prom "github.com/microsoft/retina/test/e2ev3/pkg/prometheus"
	"github.com/microsoft/retina/test/e2ev3/pkg/utils"
)

func addAdvancedDNSScenario(restConfig *rest.Config, namespace, arch, variant string,
	command string, expectError bool,
	reqQuery, reqQueryType, workloadKind string,
	respNumResponse, respQuery, respQueryType, respReturnCode, respResponse string,
) *flow.Workflow {
	wf := &flow.Workflow{DontPanic: true}
	agnhostName := "agnhost-adv-dns-" + variant + "-" + arch
	podName := agnhostName + "-0"

	createAgnhost := &k8s.CreateAgnhostStatefulSet{
		AgnhostName: agnhostName, AgnhostNamespace: namespace, AgnhostArch: arch, RestConfig: restConfig,
	}
	// Generate traffic inside the validation loop so packetparser captures it.
	execTraffic := flow.Func("adv-dns-"+variant+"-traffic-"+arch, func(ctx context.Context) error {
		exec := &k8s.ExecInPod{PodName: podName, PodNamespace: namespace, Command: command, RestConfig: restConfig}
		for i := 0; i < 2; i++ {
			if err := exec.Do(ctx); err != nil && !expectError {
				return err
			}
		}
		return nil
	})
	validateReq := &ValidateAdvancedDNSRequestStep{
		PodNamespace: namespace, PodName: podName, Query: reqQuery, QueryType: reqQueryType,
		WorkloadKind: workloadKind, WorkloadName: agnhostName, RestConfig: restConfig,
	}
	validateResp := &ValidateAdvancedDNSResponseStep{
		PodNamespace: namespace, NumResponse: respNumResponse, PodName: podName,
		Query: respQuery, QueryType: respQueryType, Response: respResponse, ReturnCode: respReturnCode,
		WorkloadKind: workloadKind, WorkloadName: agnhostName, RestConfig: restConfig,
	}
	validateWithPF := &utils.WithPortForward{
		PF: &k8s.PortForward{
			Namespace: config.KubeSystemNamespace, LabelSelector: "k8s-app=retina",
			LocalPort: config.RetinaMetricsPort, RemotePort: config.RetinaMetricsPort,
			Endpoint: "metrics", RestConfig: restConfig, OptionalLabelAffinity: "app=" + agnhostName,
		},
		Steps: []flow.Steper{execTraffic, validateReq, validateResp},
	}
	deleteAgnhost := &k8s.DeleteKubernetesResource{
		ResourceType: k8s.TypeString(k8s.StatefulSet), ResourceName: agnhostName, ResourceNamespace: namespace, RestConfig: restConfig,
	}

	// Setup: provision the agnhost pod.
	wf.Add(
		flow.Step(createAgnhost).
			Timeout(utils.DefaultScenarioTimeout),
	)

	// Validate: generate traffic + check metrics, retrying with backoff.
	wf.Add(
		flow.Step(validateWithPF).
			DependsOn(createAgnhost).
			Retry(utils.RetryWithBackoff),
	)

	// Cleanup: always runs, even if validation fails.
	wf.Add(
		flow.Pipe(deleteAgnhost).
			DependsOn(validateWithPF).
			When(flow.Always),
	)
	return wf
}



// EmptyResponse is a sentinel value that gets converted to an empty string
// for metric label matching.
const EmptyResponse = "emptyResponse"

// KubeServiceIP is a sentinel value that gets resolved at runtime to the
// ClusterIP of the kubernetes.default service.
const KubeServiceIP = "kubeServiceIP"

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
	RestConfig *rest.Config
}

func (v *ValidateAdvancedDNSRequestStep) Do(ctx context.Context) error {
	metricsEndpoint := fmt.Sprintf("http://localhost:%s/metrics", config.RetinaMetricsPort)

	podIP, err := k8s.GetPodIP(ctx, v.RestConfig, v.PodNamespace, v.PodName)
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

	err = prom.CheckMetric(ctx, metricsEndpoint, dnsAdvRequestCountMetricName, validateAdvancedDNSRequestMetrics)
	if err != nil {
		return fmt.Errorf("failed to verify advance dns request metrics %s: %w", dnsAdvRequestCountMetricName, err)
	}
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
	RestConfig *rest.Config
}

func (v *ValidateAdvancedDNSResponseStep) Do(ctx context.Context) error {
	metricsEndpoint := fmt.Sprintf("http://localhost:%s/metrics", config.RetinaMetricsPort)

	podIP, err := k8s.GetPodIP(ctx, v.RestConfig, v.PodNamespace, v.PodName)
	if err != nil {
		return fmt.Errorf("failed to get pod IP address: %w", err)
	}

	if v.Response == EmptyResponse {
		v.Response = ""
	}
	if v.Response == KubeServiceIP {
		clientset, err := kubernetes.NewForConfig(v.RestConfig)
		if err != nil {
			return fmt.Errorf("failed to create kubernetes clientset: %w", err)
		}
		svc, err := clientset.CoreV1().Services("default").Get(ctx, "kubernetes", metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get kubernetes service ClusterIP: %w", err)
		}
		v.Response = svc.Spec.ClusterIP
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

	err = prom.CheckMetric(ctx, metricsEndpoint, dnsAdvResponseCountMetricName, validateAdvanceDNSResponseMetrics)
	if err != nil {
		return fmt.Errorf("failed to verify advance dns response metrics %s: %w", dnsAdvResponseCountMetricName, err)
	}
	return nil
}
