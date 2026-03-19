// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package flags

import (
	flow "github.com/Azure/go-workflow"
	"github.com/microsoft/retina/test/e2ev3/framework/generic"
)

// LoadGenericFlags creates a workflow that loads image registry, namespace,
// and tag environment variables.
func LoadGenericFlags() *flow.Workflow {
	wf := new(flow.Workflow)

	loadFlags := &generic.LoadFlags{
		TagEnv:            generic.DefaultTagEnv,
		ImageNamespaceEnv: generic.DefaultImageNamespace,
		ImageRegistryEnv:  generic.DefaultImageRegistry,
	}

	wf.Add(flow.Step(loadFlags))
	return wf
}
