// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package capture

import (
	"github.com/microsoft/retina/test/e2e/framework/types"
	"k8s.io/apimachinery/pkg/util/rand"
)

func ValidateCapture(kubeConfigPath, namespace string) *types.Scenario {
	scenarioName := "Retina Capture"
	captureName := "retina-capture-e2e-" + rand.String(5)
	sourceDestIPsCaptureName := "retina-capture-e2e-src-dst-ip-" + rand.String(5)
	steps := []*types.StepWrapper{
		{
			Step: &InstallRetinaPlugin{},
		},
		{
			Step: &validateCapture{
				CaptureName:      captureName,
				CaptureNamespace: namespace,
				Duration:         "5s",
				KubeConfigPath:   kubeConfigPath,
				SourceIPs:        noIPFilter,
				DestinationIPs:   noIPFilter,
			}, Opts: &types.StepOptions{
				SkipSavingParametersToJob: true,
			},
		},
		{
			// Exercises --source-ips/--destination-ips end-to-end (CLI parsing, CRD validation,
			// BPF filter generation, job execution); doesn't assert on captured packet content.
			Step: &validateCapture{
				CaptureName:      sourceDestIPsCaptureName,
				CaptureNamespace: namespace,
				Duration:         "5s",
				KubeConfigPath:   kubeConfigPath,
				SourceIPs:        "127.0.0.1",
				DestinationIPs:   "127.0.0.1",
			}, Opts: &types.StepOptions{
				SkipSavingParametersToJob: true,
			},
		},
	}
	return types.NewScenario(scenarioName, steps...)
}
