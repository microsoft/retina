// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package hubblemetrics

import (
	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/steps"
)

func addHubbleRelayValidation(wf *flow.Workflow, upstream flow.Steper, kubeConfigFilePath string) flow.Steper {
	validateRelay := &steps.ValidateHubbleRelayServiceStep{KubeConfigFilePath: kubeConfigFilePath}
	wf.Add(flow.Step(validateRelay).DependsOn(upstream))
	return validateRelay
}

func addHubbleUIValidation(wf *flow.Workflow, upstream flow.Steper, kubeConfigFilePath string) flow.Steper {
	validateUI := &steps.ValidateHubbleUIServiceStep{KubeConfigFilePath: kubeConfigFilePath}
	wf.Add(flow.Step(validateUI).DependsOn(upstream))
	return validateUI
}
