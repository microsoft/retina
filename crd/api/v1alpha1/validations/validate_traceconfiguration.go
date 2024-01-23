/*
Copyright (c) Microsoft Corporation.
Licensed under the MIT license.
*/

package validations

import (
	"fmt"
	"strconv"

	"github.com/microsoft/retina/crd/api/v1alpha1"
)

func TracesCRD(tracesConfig *v1alpha1.TracesConfiguration) error {
	if tracesConfig == nil {
		return fmt.Errorf("trace configuration is nil")
	}

	if tracesConfig.Name == "" {
		return fmt.Errorf("trace configuration name is empty")
	}

	if tracesConfig.Spec == nil {
		return fmt.Errorf("trace configuration spec is nil")
	}

	if tracesConfig.Spec.TraceConfiguration == nil {
		return fmt.Errorf("trace configuration spec is nil")
	}

	if len(tracesConfig.Spec.TraceConfiguration) == 0 {
		return fmt.Errorf("trace configuration spec is empty")
	}

	err := TraceConfiguration(tracesConfig.Spec.TraceConfiguration)
	if err != nil {
		return err
	}

	err = TraceOutputConfiguration(tracesConfig.Spec.TraceOutputConfiguration)
	if err != nil {
		return err
	}

	if tracesConfig.Status == nil || tracesConfig.Status.LastKnownSpec == nil {
		// TODO add status validation
		return nil
	}

	err = TraceConfiguration(tracesConfig.Status.LastKnownSpec.TraceConfiguration)
	if err != nil {
		return err
	}

	return nil
}

// TraceConfiguration validates the trace configuration
func TraceConfiguration(traceConfig []*v1alpha1.TraceConfiguration) error {
	for _, trace := range traceConfig {
		if (trace.TraceCaptureLevel != v1alpha1.AllPacketsCapture) && (trace.TraceCaptureLevel != v1alpha1.FirstPacketCapture) {
			return fmt.Errorf("%s trace capture level is invalid", trace.TraceCaptureLevel)
		}

		if trace.TraceTargets == nil {
			return fmt.Errorf("trace targets is nil")
		}

		if len(trace.TraceTargets) == 0 {
			return fmt.Errorf("trace targets is empty")
		}

		for _, targets := range trace.TraceTargets {
			err := TraceTargets(targets)
			if err != nil {
				return err
			}
		}

	}

	return nil
}

func TraceOutputConfiguration(traceOutputConfig *v1alpha1.TraceOutputConfiguration) error {
	if traceOutputConfig == nil {
		return fmt.Errorf("trace output configuration is nil")
	}

	if traceOutputConfig.TraceOutputDestination == "" {
		return fmt.Errorf("trace output type is invalid")
	}

	// TODO add actual output type validation

	return nil
}

func TraceTargets(target *v1alpha1.TraceTargets) error {
	if target == nil {
		return fmt.Errorf("trace target is nil")
	}

	if target.Source == nil && target.Destination == nil && len(target.Ports) == 0 {
		return fmt.Errorf("trace target is empty")
	}

	err := TracePoints(target.TracePoints)
	if err != nil {
		return err
	}

	err = TraceTarget(target.Source)
	if err != nil {
		return err
	}

	err = TraceTarget(target.Destination)
	if err != nil {
		return err
	}

	for _, port := range target.Ports {
		if port == nil {
			continue
		}
		err = TracePort(port)
		if err != nil {
			return err
		}
	}

	return nil
}

func TracePoints(tp v1alpha1.TracePoints) error {
	for _, tracePoint := range tp {
		if tracePoint != v1alpha1.PodToNode &&
			tracePoint != v1alpha1.NodeToPod &&
			tracePoint != v1alpha1.NodeToNetwork &&
			tracePoint != v1alpha1.NetworkToNode {
			return fmt.Errorf("invalid trace point %s", tracePoint)
		}
	}

	return nil
}

func TraceTarget(tt *v1alpha1.TraceTarget) error {
	if tt == nil {
		return nil
	}

	// Atleast one of the selector must be present
	// Only one of the followin group of selectors can be present
	// 1. IPBlock
	// 2. NamespaceSelector, PodSelector // NamespaceSelector is mandatory if PodSelector is present
	// 3. NodeSelector
	// 4. ServiceSelector
	if tt.IPBlock.IsEmpty() &&
		tt.NamespaceSelector == nil &&
		tt.PodSelector == nil &&
		tt.NodeSelector == nil &&
		tt.ServiceSelector == nil {
		return fmt.Errorf("trace target must have at least one selector")
	}

	if !tt.IPBlock.IsEmpty() &&
		(tt.NamespaceSelector != nil || tt.PodSelector != nil || tt.NodeSelector != nil || tt.ServiceSelector != nil) {
		return fmt.Errorf("trace target cannot have both IPBlock and other selectors")
	}

	if tt.NamespaceSelector == nil && tt.PodSelector != nil {
		return fmt.Errorf("trace target must have namespace selector if pod selector is present")
	}

	if tt.NamespaceSelector != nil &&
		(tt.NodeSelector != nil || tt.ServiceSelector != nil) {
		return fmt.Errorf("trace target cannot have node or service selector if namespace and pod selector is present")
	}

	if tt.NamespaceSelector == nil && tt.PodSelector == nil &&
		(tt.NodeSelector != nil || tt.ServiceSelector != nil) &&
		(tt.NodeSelector != nil && tt.ServiceSelector != nil) {
		return fmt.Errorf("trace target cannot have both node and service selector")
	}

	return nil
}

func TracePort(port *v1alpha1.TracePorts) error {
	startPort, err := strconv.Atoi(port.Port)
	if err != nil {
		return fmt.Errorf("invalid port %s", port.Port)
	}

	if startPort < 0 || startPort > 65535 {
		return fmt.Errorf("invalid port %s", port.Port)
	}

	if port.EndPort != "" && port.EndPort != "0" {
		endPort, err := strconv.Atoi(port.EndPort)
		if err != nil {
			return fmt.Errorf("invalid end port %s", port.EndPort)
		}

		if endPort < 0 || endPort > 65535 {
			return fmt.Errorf("invalid end port %s", port.EndPort)
		}

		if endPort < startPort {
			return fmt.Errorf("invalid end port %s compared to starting port %s", port.EndPort, port.Port)
		}
	}

	return nil
}
