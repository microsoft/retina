// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package basicmetrics

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/microsoft/retina/test/e2ev3/config"
	k8s "github.com/microsoft/retina/test/e2ev3/pkg/kubernetes"
	prom "github.com/microsoft/retina/test/e2ev3/pkg/prometheus"
	"github.com/microsoft/retina/test/retry"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

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
	RestConfig              *rest.Config
	RetinaDaemonSetNamespace string
	RetinaDaemonSetName      string
}

func (v *ValidateHNSMetricStep) String() string { return "validate-hns-metrics" }

func (v *ValidateHNSMetricStep) Do(ctx context.Context) error {
	log := slog.With("step", v.String())
	clientset, err := kubernetes.NewForConfig(v.RestConfig)
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

	log.Info("checking for metric", "metric", hnsMetricName, "labels", labels)

	err = defaultRetrier.Do(ctx, func() error {
		output, execErr := k8s.ExecPod(ctx, clientset, v.RestConfig, windowsRetinaPod.Namespace, windowsRetinaPod.Name, fmt.Sprintf("curl -s http://localhost:%s/metrics", config.RetinaMetricsPort))
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

	log.Info("found matching metric", "metric", hnsMetricName, "labels", labels)
	return nil
}
