package retina

import (
	"testing"

	"github.com/microsoft/retina/test/e2e/framework/types"
	jobs "github.com/microsoft/retina/test/e2e/jobs"
	"github.com/stretchr/testify/require"
)

// TestE2ERetina tests all e2e scenarios for retina
func TestLongRunningRetina(t *testing.T) {
	settings, err := LoadInfraSettings()
	require.NoError(t, err)

	// CreateTestInfra
	createTestInfra := types.NewRunner(t, jobs.CreateTestInfra(subID, clusterName, location, settings.KubeConfigFilePath, settings.CreateInfra))
	createTestInfra.Run()
	t.Cleanup(func() {
		if settings.DeleteInfra {
			_ = jobs.DeleteTestInfra(subID, clusterName, location).Run()
		}
	})

	longrunning := types.NewRunner(t, jobs.CreateLongRunningTest(subID, clusterName, location, settings.KubeConfigFilePath, settings.CreateInfra))
	longrunning.Run()
}
