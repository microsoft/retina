//go:build perf

package retina

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/microsoft/retina/test/e2e/common"
	"github.com/microsoft/retina/test/e2e/framework/helpers"
	"github.com/microsoft/retina/test/e2e/framework/types"
	"github.com/microsoft/retina/test/e2e/infra"
	jobs "github.com/microsoft/retina/test/e2e/jobs"
	"github.com/stretchr/testify/require"
)

var (
	// Add flags for the test
	// retina-mode: basic, advanced, hubble
	retinaMode = flag.String("retina-mode", "basic", "One of basic or advanced")
)

func validateRetinaMode(t *testing.T) {
	switch *retinaMode {
	case "basic", "advanced":
		fmt.Printf("Running retina in %s mode\n", *retinaMode)
	default:
		require.Fail(t, "invalid retina-mode", "must be one of: basic, advanced")
	}
}

// This test creates a new k8s cluster runs some network performance tests
// saves the data as benchmark information and then installs retina and runs the performance tests
// to compare the results and publishes a json with regression information.
func TestE2EPerfRetina(t *testing.T) {
	validateRetinaMode(t)

	ctx, cancel := helpers.Context(t)
	defer cancel()

	cwd, err := os.Getwd()
	require.NoError(t, err)

	// Get to root of the repo by going up two directories
	rootDir := filepath.Dir(filepath.Dir(cwd))

	err = jobs.LoadGenericFlags().Run()
	require.NoError(t, err, "failed to load generic flags")

	if *common.KubeConfig == "" {
		*common.KubeConfig = infra.CreateAzureTempK8sInfra(ctx, t, rootDir)
	}

	appInsightsKey := os.Getenv(common.AzureAppInsightsKeyEnv)
	if appInsightsKey == "" {
		t.Log("No app insights key provided, results will be saved locally at ./ as `netperf-benchmark-*`, `netperf-result-*`, and `netperf-regression-*`")
	}

	// Gather benchmark results then install retina and run the performance tests
	runner := types.NewRunner(t, jobs.RunPerfTest(*common.KubeConfig, common.RetinaChartPath(rootDir), common.RetinaAdvancedProfilePath(rootDir), *retinaMode))
	runner.Run(ctx)
}
