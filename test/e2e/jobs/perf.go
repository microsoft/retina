package retina

import (
	"fmt"
	"time"

	"github.com/microsoft/retina/test/e2e/framework/generic"
	"github.com/microsoft/retina/test/e2e/framework/kubernetes"
	"github.com/microsoft/retina/test/e2e/framework/types"
	"github.com/microsoft/retina/test/e2e/scenarios/perf"
)

func RunPerfTest(kubeConfigFilePath, chartPath, advancedValuePath, retinaMode string) *types.Job {
	job := types.NewJob("Run performance tests")

	baselineFile := fmt.Sprintf("netperf-baseline-%s.json", time.Now().Format("20060102150405"))
	resultFile := fmt.Sprintf("netperf-result-%s.json", time.Now().Format("20060102150405"))
	deltaFile := fmt.Sprintf("netperf-delta-%s.json", time.Now().Format("20060102150405"))

	job.AddStep(&perf.GetNetworkPerformanceMeasures{
		KubeConfigFilePath: kubeConfigFilePath,
		ResultTag:          "no-retina",
		JsonOutputFile:     baselineFile,
	}, &types.StepOptions{
		SkipSavingParametersToJob: true,
	})

	job.AddStep(&kubernetes.InstallHelmChart{
		Namespace:          "kube-system",
		ReleaseName:        "retina",
		KubeConfigFilePath: kubeConfigFilePath,
		ChartPath:          chartPath,
		TagEnv:             generic.DefaultTagEnv,
	}, nil)

	if retinaMode == "advanced" {
		job.AddStep(&kubernetes.UpgradeRetinaHelmChart{
			Namespace:          "kube-system",
			ReleaseName:        "retina",
			KubeConfigFilePath: kubeConfigFilePath,
			ChartPath:          chartPath,
			ValuesFile:         advancedValuePath,
			TagEnv:             generic.DefaultTagEnv,
		}, &types.StepOptions{
			SkipSavingParametersToJob: true,
		})
	}

	job.AddStep(&perf.GetNetworkPerformanceMeasures{
		KubeConfigFilePath: kubeConfigFilePath,
		ResultTag:          "retina",
		JsonOutputFile:     resultFile,
	}, &types.StepOptions{
		SkipSavingParametersToJob: true,
	})

	job.AddStep(&perf.GetNetworkDeltaResults{
		BaseResultsFile:  baselineFile,
		NewResultsFile:   resultFile,
		DeltaResultsFile: deltaFile,
	}, &types.StepOptions{
		SkipSavingParametersToJob: true,
	})

	job.AddStep(&perf.PublishPerfResults{
		ResultsFile: deltaFile,
		RetinaMode:  retinaMode,
	}, &types.StepOptions{
		SkipSavingParametersToJob: true,
	})

	return job
}
