package retina

import (
	"testing"

	"github.com/microsoft/retina/test/e2e/framework/helpers"
	"github.com/microsoft/retina/test/e2e/framework/types"
	jobs "github.com/microsoft/retina/test/e2e/jobs"
	"github.com/stretchr/testify/require"
)

// Scrape PProf over a long running datapath tests
func TestLongRunningRetina(t *testing.T) {
	ctx, cancel := helpers.Context(t)
	defer cancel()

	settings, err := LoadInfraSettings()
	require.NoError(t, err)

	// CreateTestInfra
	createTestInfra := types.NewRunner(t, jobs.CreateTestInfra(subID, resourceGroup, clusterName, location, settings.KubeConfigFilePath, settings.CreateInfra))
	createTestInfra.Run(ctx)
	t.Cleanup(func() {
		if settings.DeleteInfra {
			_ = jobs.DeleteTestInfra(subID, resourceGroup, clusterName, location).Run()
		}
	})

	longrunning := types.NewRunner(t, jobs.CreateLongRunningTest(subID, clusterName, location, settings.KubeConfigFilePath, settings.CreateInfra))
	longrunning.Run(ctx)
}
