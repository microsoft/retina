//go:build !ignore_uncovered
// +build !ignore_uncovered

/*
Copyright (c) Microsoft Corporation.
Licensed under the MIT license.
*/

package v1alpha1

import (
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.
// Important: Run "make" to regenerate code after modifying this file

const (
	StateInitialized string = "Initialized"
	StateAccepted    string = "Accepted"
	StateErrored     string = "Errored"
	StateWarning     string = "Warning"
)

// MetricsContextOptions indicates the configuration for retina plugin metrics
type MetricsContextOptions struct {
	// MetricName indicates the name of the metric
	MetricName string `json:"metricName"`
	// SourceLabels represents the source context of the metrics collected
	// Such as IP, pod, port
	// +listType=set
	SourceLabels []string `json:"sourceLabels,omitempty"`
	// DestinationLabels represents the destination context of the metrics collected
	// Such as IP, pod, port, workload (deployment/replicaset/statefulset/daemonset)
	// +listType=set
	DestinationLabels []string `json:"destinationLabels,omitempty"`
	// AdditionalContext represents the additional context of the metrics collected
	// Such as Direction (ingress/egress)
	// +optional
	// +listType=set
	AdditionalLabels []string `json:"additionalLabels,omitempty"`
}

// MetricsNamespaces indicates the namespaces to include or exclude in metric collection
type MetricsNamespaces struct {
	// +listType=set
	Include []string `json:"include,omitempty"`
	// +listType=set
	Exclude []string `json:"exclude,omitempty"`
}

// Specification of the desired behavior of the RetinaMetrics. Can be omitted because this is for advanced metrics.
type MetricsSpec struct {
	ContextOptions []MetricsContextOptions `json:"contextOptions"`
	Namespaces     MetricsNamespaces       `json:"namespaces"`
}

type MetricsStatus struct {
	// +kubebuilder:validation:Enum=Initialized;Accepted;Errored;Warning
	// +kubebuilder:default:="Initialized"
	State         string       `json:"state"`
	Reason        string       `json:"reason"`
	LastKnownSpec *MetricsSpec `json:"lastKnownSpec,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:categories={retina},scope=Cluster
// +kubebuilder:subresource:status

// MetricsConfiguration contains the specification for the retina plugin metrics
type MetricsConfiguration struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec MetricsSpec `json:"spec"`
	// +optional
	Status MetricsStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MetricsConfigurationList contains a list of MetricsConfiguration
type MetricsConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MetricsConfiguration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MetricsConfiguration{}, &MetricsConfigurationList{})
}

func (m *MetricsContextOptions) IsAdvanced() bool {
	return m != nil &&
		m.MetricName != "" && (len(m.SourceLabels) > 0 ||
		len(m.DestinationLabels) > 0)
}

func (m *MetricsSpec) WithIncludedNamespaces(namespaces []string) *MetricsSpec {
	m.Namespaces.Include = namespaces
	return m
}

func (m *MetricsSpec) WithMetricsContextOptions(metrics, srcLabels, dstLabels []string) *MetricsSpec {
	for _, metric := range metrics {
		m.ContextOptions = append(m.ContextOptions, MetricsContextOptions{
			MetricName:        metric,
			SourceLabels:      srcLabels,
			DestinationLabels: dstLabels,
		})
	}
	return m
}

func (m *MetricsSpec) Equals(other *MetricsSpec) bool {
	return reflect.DeepEqual(m, other)
}
