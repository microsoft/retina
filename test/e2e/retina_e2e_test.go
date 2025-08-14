//go:build e2e

package retina

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/microsoft/retina/test/e2e/common"
	"github.com/microsoft/retina/test/e2e/framework/helpers"
	"github.com/microsoft/retina/test/e2e/framework/kubernetes"
	"github.com/microsoft/retina/test/e2e/framework/types"
	"github.com/microsoft/retina/test/e2e/infra"
	jobs "github.com/microsoft/retina/test/e2e/jobs"
	"github.com/stretchr/testify/require"
)

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

	// Wait for the HPC pod to be ready. Maximum wait time is 15 minutes in case the Pods are very slow to come up.
	err = kubernetes.WaitForPodReadyWithTimeOut(context.TODO(), common.KubeConfigFilePath(rootDir), "install-ebpf-xdp", "name=install-ebpf-xdp", 15*time.Minute)
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

	// Install Retina basic and test captures
	captureE2E := types.NewRunner(t,
		jobs.ValidateCapture(
			common.KubeConfigFilePath(rootDir),
			"default"),
	)
	captureE2E.Run(ctx)
}
