// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package utils

import (
	"fmt"
	"reflect"
	"strings"
)

// StepName derives a kebab-case step name from s.
// If s implements fmt.Stringer, its String() value is used directly.
// Otherwise the concrete type name is converted to kebab-case:
// *k8s.CreateNamespace → "create-namespace".
// Generic names like "Workflow" or "Step" are replaced by the package name:
// *basicmetrics.Workflow → "basic-metrics", *config.Step → "config".
func StepName(s any) string {
	if str, ok := s.(fmt.Stringer); ok {
		return str.String()
	}
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
