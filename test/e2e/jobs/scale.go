package retina

import (
	"os"

	"github.com/microsoft/retina/test/e2e/framework/scaletest"
	"github.com/microsoft/retina/test/e2e/framework/types"
)

func DefaultScaleTestOptions() scaletest.Options {
	return scaletest.Options{
		LabelsToGetMetrics:          map[string]string{},
		AdditionalTelemetryProperty: map[string]string{},
	}
}

func ScaleTest(opt *scaletest.Options) *types.Job {
	job := types.NewJob("Scale Test")

	job.AddStep(&scaletest.GetAndPublishMetrics{
		KubeConfigFilePath:          opt.KubeconfigPath,
		Labels:                      opt.LabelsToGetMetrics,
		AdditionalTelemetryProperty: opt.AdditionalTelemetryProperty,
		OutputFilePath:              os.Getenv("OUTPUT_FILEPATH"),
	}, &types.StepOptions{
		SkipSavingParametersToJob: true,
		RunInBackgroundWithID:     "metrics",
	})

	job.AddStep(&scaletest.ClusterLoader2{}, nil)

	job.AddStep(&types.Stop{
		BackgroundID: "metrics",
	}, nil)

	return job
}
