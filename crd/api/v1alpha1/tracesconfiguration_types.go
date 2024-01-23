//go:build !ignore_uncovered
// +build !ignore_uncovered

/*
Copyright (c) Microsoft Corporation.
Licensed under the MIT license.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.
// Important: Run "make" to regenerate code after modifying this file

const (
	AllPacketsCapture  string = "AllPackets"
	FirstPacketCapture string = "FirstPacket"
	PodToNode          string = "PodToNode"
	NodeToPod          string = "NodeToPod"
	NodeToNetwork      string = "NodeToNetwork"
	NetworkToNode      string = "NetworkToNode"
)

type TracePoints []string

type TracePorts struct {
	// +kubebuilder:validation:XIntOrString
	Port string `json:"port"`
	// +kubebuilder:default=TCP
	Protocol string `json:"protocol"`
	// +optional
	// +kubebuilder:validation:XIntOrString
	EndPort string `json:"endPort"`
}

type IPBlock struct {
	// +optional
	CIDR string `json:"cidr"`
	// +optional
	Except []string `json:"except"`
}

type TraceTarget struct {
	// +optional
	IPBlock IPBlock `json:"ipBlock"`
	// +optional
	NamespaceSelector *metav1.LabelSelector `json:"namespaceSelector"`
	// +optional
	PodSelector *metav1.LabelSelector `json:"podSelector"`
	// +optional
	NodeSelector *metav1.LabelSelector `json:"nodeSelector"`
	// +optional
	ServiceSelector *metav1.LabelSelector `json:"serviceSelector"`
}

type TraceTargets struct {
	// +optional
	Source *TraceTarget `json:"from"`
	// +optional
	Destination *TraceTarget `json:"to"`
	// +optional
	Ports []*TracePorts `json:"ports"`
	// +optional
	TracePoints TracePoints `json:"tracePoints"`
}

// TraceConfiguration indicates the configuration for retina traces
type TraceConfiguration struct {
	// +kubebuilder:validation:Enum=AllPackets;FirstPacket
	TraceCaptureLevel string          `json:"captureLevel"`
	IncludeLayer7Data bool            `json:"includeLayer7Data"`
	TraceTargets      []*TraceTargets `json:"traceTargets"`
}

// TraceOutputConfiguration indicates the configuration for retina traces outputs
type TraceOutputConfiguration struct {
	// +kubebuilder:validation:Enum=stdout;azuretable;loganalytics;opentelemetry
	TraceOutputDestination  string `json:"destination"`
	ConnectionConfiguration string `json:"connectionConfiguration"`
}

// Specification of the desired behavior of the RetinaTraces. Can be omitted because this is for advanced Traces.
type TracesSpec struct {
	TraceConfiguration       []*TraceConfiguration     `json:"traceConfiguration"`
	TraceOutputConfiguration *TraceOutputConfiguration `json:"outputConfiguration"`
}

type TracesStatus struct {
	// +kubebuilder:validation:Enum=Initialized;Accepted;Errored;Warning
	// +kubebuilder:default:="Initialized"
	State         string      `json:"state"`
	Reason        string      `json:"reason"`
	LastKnownSpec *TracesSpec `json:"lastKnownSpec"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:categories={retina},scope=Cluster
// +kubebuilder:subresource:status

// TracesConfiguration contains the specification for the retina plugin Traces
type TracesConfiguration struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec *TracesSpec `json:"spec"`

	Status *TracesStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// TracesConfigurationList contains a list of TracesConfigurationList
type TracesConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TracesConfigurationList `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TracesConfiguration{}, &TracesConfigurationList{})
}

func (tc *TraceConfiguration) Equal(new *TraceConfiguration) bool {
	if tc == nil && new == nil {
		return true
	}

	if tc == nil || new == nil {
		return false
	}

	if tc.TraceCaptureLevel != new.TraceCaptureLevel {
		return false
	}

	if tc.IncludeLayer7Data != new.IncludeLayer7Data {
		return false
	}

	if len(tc.TraceTargets) != len(new.TraceTargets) {
		return false
	}

	for i := range tc.TraceTargets {
		if !tc.TraceTargets[i].Equal(new.TraceTargets[i]) {
			return false
		}
	}

	return true
}

func (tt *TraceTargets) Equal(new *TraceTargets) bool {
	if tt == nil && new == nil {
		return true
	}

	if tt == nil || new == nil {
		return false
	}

	if !tt.Source.Equal(new.Source) {
		return false
	}

	if !tt.Destination.Equal(new.Destination) {
		return false
	}

	if len(tt.Ports) != len(new.Ports) {
		return false
	}

	for i := range tt.Ports {
		if !tt.Ports[i].Equal(new.Ports[i]) {
			return false
		}
	}

	if len(tt.TracePoints) != len(new.TracePoints) {
		return false
	}

	for i := range tt.TracePoints {
		if tt.TracePoints[i] != new.TracePoints[i] {
			return false
		}
	}

	return true
}

func (tp *TracePorts) Equal(new *TracePorts) bool {
	if tp == nil && new == nil {
		return true
	}

	if tp == nil || new == nil {
		return false
	}

	if tp.Port != new.Port {
		return false
	}

	if tp.Protocol != new.Protocol {
		return false
	}

	if tp.EndPort != new.EndPort {
		return false
	}

	return true
}

func (tt *TraceTarget) Equal(new *TraceTarget) bool {
	if tt == nil && new == nil {
		return true
	}

	if tt == nil || new == nil {
		return false
	}

	if !tt.IPBlock.Equal(&new.IPBlock) {
		return false
	}

	if tt.NamespaceSelector.String() != new.NamespaceSelector.String() {
		return false
	}

	if tt.PodSelector.String() != new.PodSelector.String() {
		return false
	}

	if tt.NodeSelector.String() != new.NodeSelector.String() {
		return false
	}

	if tt.ServiceSelector.String() != new.ServiceSelector.String() {
		return false
	}

	return true
}

func (ip *IPBlock) Equal(new *IPBlock) bool {
	if ip == nil && new == nil {
		return true
	}

	if ip == nil || new == nil {
		return false
	}

	if ip.CIDR != new.CIDR {
		return false
	}

	if len(ip.Except) != len(new.Except) {
		return false
	}

	for i := range ip.Except {
		if ip.Except[i] != new.Except[i] {
			return false
		}
	}

	return true
}

func (ip *IPBlock) IsEmpty() bool {
	// if ip is nil or CIDR is empty, then it is empty
	// even if except is populated with CIDR being empty that is invalid
	return ip == nil || ip.CIDR == ""
}
