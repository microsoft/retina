// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package capture

import (
	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/steps"
	"k8s.io/apimachinery/pkg/util/rand"
)

// ValidateCapture creates a workflow that installs the retina kubectl plugin
// and validates packet capture functionality (create, verify, download, delete).
func ValidateCapture(kubeConfigFilePath, testPodNamespace string) *flow.Workflow {
	wf := new(flow.Workflow)

	captureName := "retina-capture-e2e-" + rand.String(5)

	installPlugin := &steps.InstallRetinaPluginStep{}
	validateCap := &steps.ValidateCaptureStep{
		CaptureName:      captureName,
		CaptureNamespace: testPodNamespace,
		Duration:         "5s",
		KubeConfigPath:   kubeConfigFilePath,
	}

	wf.Add(flow.Pipe(installPlugin, validateCap))

	return wf
}
