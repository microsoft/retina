package retina

import (
	"testing"

	"github.com/microsoft/retina/test/e2e/framework/types"
	jobs "github.com/microsoft/retina/test/e2e/jobs"
)

// TestE2ERetina tests all e2e scenarios for retina
func TestE2ERetina(t *testing.T) {
	// CreateTestInfra
	createTestInfra := types.NewRunner(t, jobs.CreateTestInfra())
	createTestInfra.Run()
	// DeleteTestInfra
	deleteTestInfra := types.NewRunner(t, jobs.DeleteTestInfra())
	defer deleteTestInfra.Run()
	// Install and test Retina basic metrics
	basicMetricsE2E := types.NewRunner(t, jobs.InstallAndTestRetinaWithBasicMetrics())
	basicMetricsE2E.Run()
	// Upgrade and test Retina with advanced metrics
	advanceMetricsE2E := types.NewRunner(t, jobs.UpgradeAndTestRetinaWithAdvancedMetrics())
	advanceMetricsE2E.Run()

}
