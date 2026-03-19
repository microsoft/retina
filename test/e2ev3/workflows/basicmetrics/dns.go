// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package basicmetrics

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

func addBasicDNSScenario(wf *flow.Workflow, dependsOn flow.Steper, kubeConfigFilePath, namespace, arch, variant, command string, expectError bool) flow.Steper {
	agnhostName := "agnhost-dns-basic-" + variant + "-" + arch
	podName := agnhostName + "-0"

	createAgnhost := &k8s.CreateAgnhostStatefulSet{
		AgnhostName: agnhostName, AgnhostNamespace: namespace, AgnhostArch: arch, KubeConfigFilePath: kubeConfigFilePath,
	}
	execCmd1 := flow.Func("basic-dns-"+variant+"-1-"+arch, func(ctx context.Context) error {
		err := (&k8s.ExecInPod{PodName: podName, PodNamespace: namespace, Command: command, KubeConfigFilePath: kubeConfigFilePath}).Do(ctx)
		if expectError {
			return nil
		}
		return err
	})
	execCmd2 := flow.Func("basic-dns-"+variant+"-2-"+arch, func(ctx context.Context) error {
		err := (&k8s.ExecInPod{PodName: podName, PodNamespace: namespace, Command: command, KubeConfigFilePath: kubeConfigFilePath}).Do(ctx)
		if expectError {
			return nil
		}
		return err
	})
	validateReq := &ValidateBasicDNSRequestStep{Variant: variant + "-" + arch}
	validateResp := &ValidateBasicDNSResponseStep{Variant: variant + "-" + arch}
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
			DependsOn(dependsOn).
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



var (
	dnsBasicRequestCountMetricName  = "networkobservability_dns_request_count"
	dnsBasicResponseCountMetricName = "networkobservability_dns_response_count"
)

// ValidateBasicDNSRequestStep checks that the basic DNS request count metric exists.
type ValidateBasicDNSRequestStep struct {
	Variant string // distinguishes instances in the DAG (e.g. "valid-domain-amd64")
}

func (v *ValidateBasicDNSRequestStep) Do(_ context.Context) error {
	metricsEndpoint := fmt.Sprintf("http://localhost:%s/metrics", config.RetinaMetricsPort)

	validBasicDNSRequestMetricLabels := map[string]string{}

	err := prom.CheckMetric(metricsEndpoint, dnsBasicRequestCountMetricName, validBasicDNSRequestMetricLabels)
	if err != nil {
		return fmt.Errorf("failed to verify basic dns request metrics %s: %w", dnsBasicRequestCountMetricName, err)
	}
	log.Printf("found metrics matching %+v\n", dnsBasicRequestCountMetricName)
	return nil
}

// ValidateBasicDNSResponseStep checks that the basic DNS response count metric exists.
type ValidateBasicDNSResponseStep struct {
	Variant     string // distinguishes instances in the DAG
	NumResponse string
	Query       string
	QueryType   string
	ReturnCode  string
	Response    string
}

func (v *ValidateBasicDNSResponseStep) Do(_ context.Context) error {
	metricsEndpoint := fmt.Sprintf("http://localhost:%s/metrics", config.RetinaMetricsPort)

	if v.Response == emptyResponse {
		v.Response = ""
	}

	validBasicDNSResponseMetricLabels := map[string]string{}

	err := prom.CheckMetric(metricsEndpoint, dnsBasicResponseCountMetricName, validBasicDNSResponseMetricLabels)
	if err != nil {
		return fmt.Errorf("failed to verify basic dns response metrics %s: %w", dnsBasicResponseCountMetricName, err)
	}
	log.Printf("found metrics matching %+v\n", dnsBasicResponseCountMetricName)
	return nil
}

// emptyResponse is a sentinel value that gets converted to an empty string
// for metric label matching.
const emptyResponse = "emptyResponse"
