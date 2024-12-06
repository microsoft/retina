package retina

import (
	"github.com/microsoft/retina/test/e2e/framework/types"
	"github.com/microsoft/retina/test/e2e/scenarios/longrunning"
)

func CreateLongRunningTest(subID, clusterName, location, kubeConfigFilePath string, createInfra bool) *types.Job {
	job := types.NewJob("Run Retina over a long period of time")
	job.AddScenario(longrunning.PullPProf(kubeConfigFilePath))
	return job
}
