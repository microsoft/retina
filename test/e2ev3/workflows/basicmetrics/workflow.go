// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package basicmetrics

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/config"
	"github.com/microsoft/retina/test/e2ev3/pkg/images"
	k8s "github.com/microsoft/retina/test/e2ev3/pkg/kubernetes"
	prom "github.com/microsoft/retina/test/e2ev3/pkg/prometheus"
	"github.com/microsoft/retina/test/e2ev3/pkg/utils"
	"github.com/microsoft/retina/test/retry"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// InstallAndTestRetinaBasicMetrics creates a workflow that installs Retina
// and validates basic metrics: drop, TCP, DNS, and Windows HNS for each architecture.
func InstallAndTestRetinaBasicMetrics(kubeConfigFilePath, chartPath, testPodNamespace string, imgCfg *config.ImageConfig, helmCfg *config.HelmConfig, loader images.Loader) *flow.Workflow {
	wf := &flow.Workflow{DontPanic: true}

	installRetina := &k8s.InstallHelmChart{
		Namespace:          config.KubeSystemNamespace,
		ReleaseName:        "retina",
		KubeConfigFilePath: kubeConfigFilePath,
		ChartPath:          chartPath,
		ImageTag:           imgCfg.Tag,
		ImageRegistry:      imgCfg.Registry,
		ImageNamespace:     imgCfg.Namespace,
		HelmDriver:         helmCfg.Driver,
		ImageLoader:        loader,
	}
	wf.Add(flow.Step(installRetina))

	var scenarioTails []flow.Steper

	for _, arch := range config.Architectures {
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

		winStep := &ValidateHNSMetricStep{
			KubeConfigFilePath:       kubeConfigFilePath,
			RetinaDaemonSetNamespace: config.KubeSystemNamespace,
			RetinaDaemonSetName:      "retina-agent-win",
		}
		wf.Add(flow.Step(winStep).DependsOn(installRetina))
		scenarioTails = append(scenarioTails, winStep)
	}

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



const (
	defaultRetryDelay    = 5 * time.Second
	defaultRetryAttempts = 5
)

var (
	ErrorNoWindowsPod = errors.New("no windows retina pod found")
	ErrNoMetricFound  = fmt.Errorf("no metric found")

	hnsMetricName  = "networkobservability_windows_hns_stats"
	defaultRetrier = retry.Retrier{Attempts: defaultRetryAttempts, Delay: defaultRetryDelay}
)

// ValidateHNSMetricStep finds a Windows retina pod, curls the metrics endpoint
// inside it, and checks for the HNS stats metric with retry logic.
type ValidateHNSMetricStep struct {
	KubeConfigFilePath       string
	RetinaDaemonSetNamespace string
	RetinaDaemonSetName      string
}

func (v *ValidateHNSMetricStep) Do(_ context.Context) error {
	restConfig, err := clientcmd.BuildConfigFromFlags("", v.KubeConfigFilePath)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("error creating Kubernetes client: %w", err)
	}

	pods, err := clientset.CoreV1().Pods(v.RetinaDaemonSetNamespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: "k8s-app=retina",
	})
	if err != nil {
		return fmt.Errorf("error listing pods: %w", err)
	}

	var windowsRetinaPod *v1.Pod
	for i := range pods.Items {
		if pods.Items[i].Spec.NodeSelector["kubernetes.io/os"] == "windows" {
			windowsRetinaPod = &pods.Items[i]
		}
	}
	if windowsRetinaPod == nil {
		return ErrorNoWindowsPod
	}

	labels := map[string]string{
		"direction": "win_packets_sent_count",
	}

	log.Printf("checking for metric %s with labels %+v\n", hnsMetricName, labels)

	err = defaultRetrier.Do(context.TODO(), func() error {
		output, execErr := k8s.ExecPod(context.TODO(), clientset, restConfig, windowsRetinaPod.Namespace, windowsRetinaPod.Name, fmt.Sprintf("curl -s http://localhost:%s/metrics", config.RetinaMetricsPort))
		if execErr != nil {
			return fmt.Errorf("error executing command in windows retina pod: %w", execErr)
		}
		if len(output) == 0 {
			return ErrNoMetricFound
		}

		checkErr := prom.CheckMetricFromBuffer(output, hnsMetricName, labels)
		if checkErr != nil {
			return fmt.Errorf("failed to verify prometheus metrics: %w", checkErr)
		}

		return nil
	})
	if err != nil {
		return err
	}

	log.Printf("found metric matching %+v: with labels %+v\n", hnsMetricName, labels)
	return nil
}
