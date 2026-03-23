// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package hubblemetrics

import (
	"k8s.io/client-go/rest"
	"context"
	"fmt"

	flow "github.com/Azure/go-workflow"
	prom "github.com/microsoft/retina/test/e2ev3/pkg/prometheus"
	"github.com/microsoft/retina/test/e2ev3/config"
	k8s "github.com/microsoft/retina/test/e2ev3/pkg/kubernetes"
	"github.com/microsoft/retina/test/e2ev3/pkg/utils"
	"k8s.io/client-go/kubernetes"
)

func addHubbleFlowIntraNodeScenario(restConfig *rest.Config, arch string) *flow.Workflow {
	wf := &flow.Workflow{DontPanic: true}
	podname := "agnhost-flow-intra"
	replicas := 2
	validLabels := []map[string]string{
		{"source": config.TestPodNamespace + "/" + podname + "-0", "destination": "", "protocol": config.TCP, "subtype": "to-stack", "type": "Trace", "verdict": "FORWARDED"},
		{"source": config.TestPodNamespace + "/" + podname + "-0", "destination": "", "protocol": config.TCP, "subtype": "to-endpoint", "type": "Trace", "verdict": "FORWARDED"},
		{"source": config.TestPodNamespace + "/" + podname + "-1", "destination": "", "protocol": config.TCP, "subtype": "to-stack", "type": "Trace", "verdict": "FORWARDED"},
		{"source": config.TestPodNamespace + "/" + podname + "-1", "destination": "", "protocol": config.TCP, "subtype": "to-endpoint", "type": "Trace", "verdict": "FORWARDED"},
	}

	createAgnhost := &k8s.CreateAgnhostStatefulSet{
		AgnhostName: podname, AgnhostNamespace: config.TestPodNamespace,
		ScheduleOnSameNode: true, AgnhostReplicas: &replicas,
		AgnhostArch: arch, RestConfig: restConfig,
	}
	curlPod := &CurlPodStep{
		SrcPodName: podname + "-0", SrcPodNamespace: config.TestPodNamespace,
		DstPodName: podname + "-1", DstPodNamespace: config.TestPodNamespace,
		RestConfig: restConfig,
	}
	validateFlow := &prom.ValidateMetricStep{
		ForwardedPort: config.HubbleMetricsPort, MetricName: config.HubbleFlowMetricName,
		ValidMetrics: validLabels, ExpectMetric: true,
	}
	validateWithPF := &utils.WithPortForward{
		PF: &k8s.PortForward{
			LabelSelector: "k8s-app=retina", LocalPort: config.HubbleMetricsPort, RemotePort: config.HubbleMetricsPort,
			Endpoint: config.MetricsEndpoint, RestConfig: restConfig, OptionalLabelAffinity: "app=" + podname,
		},
		Steps: []flow.Steper{validateFlow},
	}
	deleteAgnhost := &k8s.DeleteKubernetesResource{
		ResourceType: k8s.TypeString(k8s.StatefulSet), ResourceName: podname,
		ResourceNamespace: config.TestPodNamespace, RestConfig: restConfig,
	}

	// Setup: provision resources and generate traffic.
	wf.Add(
		flow.Pipe(createAgnhost, curlPod).
			Timeout(utils.DefaultScenarioTimeout),
	)

	// Validate: retry with exponential backoff until metrics appear.
	wf.Add(
		flow.Step(validateWithPF).
			DependsOn(curlPod).
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



// CurlPodStep executes a curl command from a source pod to a destination pod
// for flow testing. It resolves the destination pod's IP and runs the command.
type CurlPodStep struct {
	SrcPodName      string
	SrcPodNamespace string
	DstPodName      string
	DstPodNamespace string
	RestConfig      *rest.Config
}

func (c *CurlPodStep) String() string { return "curl-pod" }

func (c *CurlPodStep) Do(ctx context.Context) error {
	clientset, err := kubernetes.NewForConfig(c.RestConfig)
	if err != nil {
		return fmt.Errorf("error creating Kubernetes client: %w", err)
	}

	dstPodIP, err := k8s.GetPodIP(ctx, c.RestConfig, c.DstPodNamespace, c.DstPodName)
	if err != nil {
		return fmt.Errorf("error getting pod IP: %w", err)
	}

	cmd := fmt.Sprintf("curl -s -m 5 %s:80", dstPodIP)
	_, err = k8s.ExecPod(ctx, clientset, c.RestConfig, c.SrcPodNamespace, c.SrcPodName, cmd)
	if err != nil {
		return fmt.Errorf("error executing command: %w", err)
	}
	return nil
}
