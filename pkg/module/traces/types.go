// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package traces

import api "github.com/microsoft/retina/crd/api/v1alpha1"

//go:generate mockgen -destination=mock_moduleinterface.go -copyright_file=../../lib/ignore_headers.txt -package=traces github.com/microsoft/retina/pkg/module/traces ModuleInterface

type ModuleInterface interface {
	// Run starts the trace module.
	Run()

	Reconcile(spec *api.TracesSpec) error
}
