//go:build e2e

package retina

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/microsoft/retina/test/e2e/common"
	"github.com/microsoft/retina/test/e2e/framework/helpers"
	"github.com/microsoft/retina/test/e2e/framework/types"
	"github.com/microsoft/retina/test/e2e/infra"
	jobs "github.com/microsoft/retina/test/e2e/jobs"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func waitForPodReadyWithClientGo(ctx context.Context, clientset *kubernetes.Clientset, namespace, labelSelector string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
		if err == nil && len(pods.Items) > 0 {
			for _, cond := range pods.Items[0].Status.Conditions {
				if cond.Type == "Ready" && cond.Status == "True" {
					return nil
				}
			}
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("timeout waiting for pod to become ready")
}

// TestE2ERetina tests all e2e scenarios for retina
func TestE2ERetina(t *testing.T) {
	ctx, cancel := helpers.Context(t)
	defer cancel()

	cwd, err := os.Getwd()
	require.NoError(t, err)

	// Get to root of the repo by going up two directories
	rootDir := filepath.Dir(filepath.Dir(cwd))

	hubblechartPath := filepath.Join(rootDir, "deploy", "hubble", "manifests", "controller", "helm", "retina")

	err = jobs.LoadGenericFlags().Run()
	require.NoError(t, err, "failed to load generic flags")

	if *common.KubeConfig == "" {
		*common.KubeConfig = infra.CreateAzureTempK8sInfra(ctx, t, rootDir)
	}

	// Install Ebpf and XDP
	installEbpfAndXDP := types.NewRunner(t, jobs.InstallEbpfXdp(common.KubeConfigFilePath(rootDir)))
	installEbpfAndXDP.Run(ctx)

	config, _ := clientcmd.BuildConfigFromFlags("", common.KubeConfigFilePath(rootDir))
	clientset, _ := kubernetes.NewForConfig(config)
	err = waitForPodReadyWithClientGo(ctx, clientset, "install-ebpf-xdp", "name=install-ebpf-xdp", 10*time.Minute)
	require.NoError(t, err)

	// Load and pin BPF Maps
	loadAndPinWinBPFJob := types.NewRunner(t, jobs.LoadAndPinWinBPFJob(common.KubeConfigFilePath(rootDir)))
	loadAndPinWinBPFJob.Run(ctx)

	// Install and test Retina basic metrics

	basicMetricsE2E := types.NewRunner(t,
		jobs.InstallAndTestRetinaBasicMetrics(
			common.KubeConfigFilePath(rootDir),
			common.RetinaChartPath(rootDir),
			common.TestPodNamespace),
	)
	basicMetricsE2E.Run(ctx)

	// Upgrade and test Retina with advanced metrics
	advanceMetricsE2E := types.NewRunner(t,
		jobs.UpgradeAndTestRetinaAdvancedMetrics(
			common.KubeConfigFilePath(rootDir),
			common.RetinaChartPath(rootDir),
			common.RetinaAdvancedProfilePath(rootDir),
			common.TestPodNamespace),
	)
	advanceMetricsE2E.Run(ctx)

	// unpin BPF Maps
	unloadAndPinWinBPFJob := types.NewRunner(t, jobs.UnLoadAndPinWinBPFJob(common.KubeConfigFilePath(rootDir)))
	unloadAndPinWinBPFJob.Run(ctx)

	// Install and test Hubble basic metrics
	validatehubble := types.NewRunner(t,
		jobs.ValidateHubble(
			common.KubeConfigFilePath(rootDir),
			hubblechartPath,
			common.TestPodNamespace),
	)
	validatehubble.Run(ctx)
}
