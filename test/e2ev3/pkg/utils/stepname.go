// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package utils

import "github.com/microsoft/retina/test/e2ev3/pkg/stepname"

// StepName derives a kebab-case step name from the concrete type of s.
// Re-exported from the stepname leaf package to avoid import cycles.
func StepName(s any) string {
	return stepname.StepName(s)
}
