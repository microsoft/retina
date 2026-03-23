// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package stepname

import (
	"reflect"
	"strings"
	"unicode"
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

// toKebabCase converts PascalCase to kebab-case, keeping acronyms together.
// "CreateNamespace"       → "create-namespace"
// "InstallNPM"            → "install-npm"
// "ValidateHTTPResponse"  → "validate-http-response"
func toKebabCase(s string) string {
	runes := []rune(s)
	var b strings.Builder
	for i, r := range runes {
		if unicode.IsUpper(r) {
			if i > 0 {
				prev := runes[i-1]
				nextIsLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
				if unicode.IsLower(prev) || (unicode.IsUpper(prev) && nextIsLower) {
					b.WriteByte('-')
				}
			}
			b.WriteRune(unicode.ToLower(r))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
