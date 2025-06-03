// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package capture

import (
	"github.com/microsoft/retina/test/e2e/framework/types"
	"k8s.io/apimachinery/pkg/util/rand"
)

func ValidateCaptureCreate(kubeConfigPath, namespace string) *types.Scenario {
	scenarioName := "Retina Capture"
	captureName := "retina-capture-e2e-" + rand.String(5)
	steps := []*types.StepWrapper{
		{
			Step: &validateCapture{
				CaptureName:      captureName,
				CaptureNamespace: namespace,
				Duration:         "5s",
				KubeConfigPath:   kubeConfigPath,
			}, Opts: &types.StepOptions{
				SkipSavingParametersToJob: true,
			},
		},
	}
	return types.NewScenario(scenarioName, steps...)
}
