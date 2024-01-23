/*
Copyright (c) Microsoft Corporation.
Licensed under the MIT license.
*/

package validations

import (
	"fmt"

	"github.com/microsoft/retina/crd/api/v1alpha1"
	"github.com/microsoft/retina/pkg/utils"
)

// MetricsConfiguration validates the metrics configuration
func MetricsCRD(metricsConfig *v1alpha1.MetricsConfiguration) error {
	if metricsConfig == nil {
		return fmt.Errorf("metrics configuration is nil")
	}

	if metricsConfig.Name == "" {
		return fmt.Errorf("metrics configuration name is empty")
	}

	err := MetricsSpec(metricsConfig.Spec)
	if err != nil {
		return err
	}

	return nil
}

// MetricsSpec validates the metrics spec
func MetricsSpec(metricsSpec v1alpha1.MetricsSpec) error {
	if len(metricsSpec.ContextOptions) == 0 {
		return fmt.Errorf("metrics spec context options is empty")
	}

	for _, contextOption := range metricsSpec.ContextOptions {
		if !utils.IsAdvancedMetric(contextOption.MetricName) {
			return fmt.Errorf("%s is not a valid metric", contextOption.MetricName)
		}
	}

	err := MetricsNamespaces(metricsSpec.Namespaces)
	if err != nil {
		return err
	}

	return nil
}

// MetricsNamespaces validates the metrics namespaces
func MetricsNamespaces(mn v1alpha1.MetricsNamespaces) error {
	if mn.Include == nil && mn.Exclude == nil {
		return fmt.Errorf("metrics namespaces include and exclude are both nil")
	}

	if mn.Include != nil && mn.Exclude != nil {
		return fmt.Errorf("metrics namespaces include and exclude are both not nil")
	}

	return nil
}

// CompareMetricsConfig compares two metrics configurations
func CompareMetricsConfig(old, new *v1alpha1.MetricsConfiguration) bool {
	if old == nil && new == nil {
		return true
	}

	if old == nil || new == nil {
		return false
	}

	if old.Name != new.Name {
		return false
	}

	if !MetricsSpecCompare(old.Spec, new.Spec) {
		return false
	}

	return true
}

// MetricsSpecCompare compares two metrics specs
func MetricsSpecCompare(old, new v1alpha1.MetricsSpec) bool {
	if len(old.ContextOptions) != len(new.ContextOptions) {
		return false
	}

	if !MetricsNamespacesCompare(old.Namespaces, new.Namespaces) {
		return false
	}

	if !MetricsContextOptionsCompare(old.ContextOptions, new.ContextOptions) {
		return false
	}

	return true
}

// MetricsNamespacesCompare compares two metrics namespaces
func MetricsNamespacesCompare(old, new v1alpha1.MetricsNamespaces) bool {
	if !utils.CompareStringSlice(old.Include, new.Include) {
		return false
	}

	if !utils.CompareStringSlice(old.Exclude, new.Exclude) {
		return false
	}

	return true
}

// MetricsContextOptionsCompare compares two metrics context options
func MetricsContextOptionsCompare(old, new []v1alpha1.MetricsContextOptions) bool {
	if len(old) != len(new) {
		return false
	}

	oldMap := make(map[string]v1alpha1.MetricsContextOptions)
	for _, contextOption := range old {
		oldMap[contextOption.MetricName] = contextOption
	}

	newMap := make(map[string]v1alpha1.MetricsContextOptions)
	for _, contextOption := range new {
		newMap[contextOption.MetricName] = contextOption
	}

	if len(oldMap) != len(newMap) {
		return false
	}

	for key, oldContextOption := range oldMap {
		newContextOption, ok := newMap[key]
		if !ok {
			return false
		}

		if !utils.CompareStringSlice(oldContextOption.SourceLabels, newContextOption.SourceLabels) {
			return false
		}

		if !utils.CompareStringSlice(oldContextOption.DestinationLabels, newContextOption.DestinationLabels) {
			return false
		}

		if oldContextOption.MetricName != newContextOption.MetricName {
			return false
		}

		if !utils.CompareStringSlice(oldContextOption.AdditionalLabels, newContextOption.AdditionalLabels) {
			return false
		}

	}

	return true
}
