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

// Workflow runs the basic metrics workflow.
type Workflow struct {
	Params *config.E2EParams
}

func (w *Workflow) String() string { return "basic-metrics" }

func (w *Workflow) Do(ctx context.Context) error {
	p := w.Params
	kubeConfigFilePath := p.Paths.KubeConfig
	chartPath := p.Paths.RetinaChart
	testPodNamespace := config.TestPodNamespace
	imgCfg := &p.Cfg.Image
	helmCfg := &p.Cfg.Helm
	loader := images.NewLoader(*config.Provider, p.Cfg.Azure.ClusterName)

	// Construct steps.
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

	var scenarioTails []flow.Steper
	for _, arch := range config.Architectures {
		scenarioTails = append(scenarioTails,
			addDropScenario(kubeConfigFilePath, testPodNamespace, arch),
			addTCPScenario(kubeConfigFilePath, testPodNamespace, arch),
			addBasicDNSScenario(kubeConfigFilePath, testPodNamespace, arch,
				"valid-domain", "nslookup kubernetes.default", false),
			addBasicDNSScenario(kubeConfigFilePath, testPodNamespace, arch,
				"nxdomain", "nslookup some.non.existent.domain", true),
		)
	}

	if *config.Provider != "kind" {
		scenarioTails = append(scenarioTails, &ValidateHNSMetricStep{
			KubeConfigFilePath:       kubeConfigFilePath,
			RetinaDaemonSetNamespace: config.KubeSystemNamespace,
			RetinaDaemonSetName:      "retina-agent-win",
		})
	}

	ensureStable := &k8s.EnsureStableComponent{
		PodNamespace:           config.KubeSystemNamespace,
		LabelSelector:          "k8s-app=retina",
		KubeConfigFilePath:     kubeConfigFilePath,
		IgnoreContainerRestart: false,
	}

	debug := &utils.DebugOnFailure{
		KubeConfigFilePath: kubeConfigFilePath,
		Namespace:          config.KubeSystemNamespace,
		LabelSelector:      "k8s-app=retina",
	}

	// Wire dependencies and register.
	wf := &flow.Workflow{DontPanic: true}
	wf.Add(flow.Step(installRetina))
	for _, s := range scenarioTails {
		wf.Add(flow.Step(s).DependsOn(installRetina))
	}
	wf.Add(flow.Step(ensureStable).DependsOn(scenarioTails...))
	wf.Add(flow.Step(debug).DependsOn(ensureStable).When(flow.AnyFailed))

	return wf.Do(ctx)
}



const (
	defaultRetryDelay    = 5 * time.Second
	defaultRetryAttempts = 5
)

var (
	ErrorNoWindowsPod = errors.New("no windows retina pod found")
	ErrNoMetricFound  = fmt.Errorf("no metric found")

	hnsMetricName  = "networkobservability_windows_hns_stats"
	defaultRetrier = retry.Retrier{Attempts: defaultRetryAttempts, Delay: defaultRetryDelay, ExpBackoff: true}
)

// ValidateHNSMetricStep finds a Windows retina pod, curls the metrics endpoint
// inside it, and checks for the HNS stats metric with retry logic.
type ValidateHNSMetricStep struct {
	KubeConfigFilePath       string
	RetinaDaemonSetNamespace string
	RetinaDaemonSetName      string
}

func (v *ValidateHNSMetricStep) Do(ctx context.Context) error {
	restConfig, err := clientcmd.BuildConfigFromFlags("", v.KubeConfigFilePath)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("error creating Kubernetes client: %w", err)
	}

	pods, err := clientset.CoreV1().Pods(v.RetinaDaemonSetNamespace).List(ctx, metav1.ListOptions{
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

	err = defaultRetrier.Do(ctx, func() error {
		output, execErr := k8s.ExecPod(ctx, clientset, restConfig, windowsRetinaPod.Namespace, windowsRetinaPod.Name, fmt.Sprintf("curl -s http://localhost:%s/metrics", config.RetinaMetricsPort))
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
