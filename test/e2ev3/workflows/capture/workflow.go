// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package capture

import (
	"context"

	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/config"
	"github.com/microsoft/retina/test/e2ev3/pkg/utils"
	"k8s.io/apimachinery/pkg/util/rand"
)

// Workflow runs the capture validation workflow.
type Workflow struct {
	Cfg *config.E2EConfig
}

func (w *Workflow) String() string { return "capture" }

func (w *Workflow) Do(ctx context.Context) error {
	ctx = utils.WithWorkflow(ctx, w.String())
	p := w.Cfg
	kubeConfigFilePath := p.Cluster.KubeConfigPath()
	testPodNamespace := "default"
	imgCfg := &p.Image

	wf := new(flow.Workflow)

	captureName := "retina-capture-e2e-" + rand.String(5)

	installPlugin := &InstallRetinaPluginStep{}
	validateCap := &ValidateCaptureStep{
		CaptureName:      captureName,
		CaptureNamespace: testPodNamespace,
		Duration:         "5s",
		KubeConfigPath:   kubeConfigFilePath,
		RestConfig:       p.Cluster.RestConfig(),
		ImageTag:         imgCfg.Tag,
		ImageRegistry:    imgCfg.Registry,
		ImageNamespace:   imgCfg.Namespace,
	}

	wf.Add(flow.Pipe(installPlugin, validateCap))

	return wf.Do(ctx)
}
