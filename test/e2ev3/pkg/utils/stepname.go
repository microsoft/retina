// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package utils

import (
	"reflect"
	"strings"
)

// StepName derives a kebab-case step name from the concrete type of s.
// For example, *k8s.CreateNamespace → "create-namespace".
// Generic names like "Workflow" or "Step" are replaced by the package name:
// *basicmetrics.Workflow → "basic-metrics", *config.Step → "config".
func StepName(s any) string {
	t := reflect.TypeOf(s)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	name := toKebabCase(t.Name())
	if name == "workflow" || name == "step" {
		pkg := t.PkgPath()
		if idx := strings.LastIndex(pkg, "/"); idx != -1 {
			return toKebabCase(pkg[idx+1:])
		}
	}
	return name
}
