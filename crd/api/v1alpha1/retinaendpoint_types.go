/*
Copyright (c) Microsoft Corporation.
Licensed under the MIT license.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=ke
// +kubebuilder:printcolumn:name="Pod IP",type=string,priority=0,JSONPath=`.spec.podIP`
// +kubebuilder:printcolumn:name="Referenced By",type=string,priority=1,JSONPath=`.spec.ownerReferences`
// RetinaEndpoint is the Schema for the retinaendpoints API
type RetinaEndpoint struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RetinaEndpointSpec   `json:"spec,omitempty"`
	Status RetinaEndpointStatus `json:"status,omitempty"`
}

// RetinaEndpointSpec defines the desired state of RetinaEndpoint
type RetinaEndpointSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Containers      []RetinaEndpointStatusContainers `json:"containers,omitempty"`
	OwnerReferences []OwnerReference                 `json:"ownerReferences,omitempty"`
	NodeIP          string                           `json:"nodeIP,omitempty"`
	PodIP           string                           `json:"podIP,omitempty"`
	PodIPs          []string                         `json:"podIPs,omitempty"`
	Labels          map[string]string                `json:"labels,omitempty"`
	Annotations     map[string]string                `json:"annotations,omitempty"`
}

type Containers struct {
	Name string `json:"name,omitempty"`
	ID   string `json:"id,omitempty"`
}

type RetinaEndpointStatusContainers struct {
	Name string `json:"name,omitempty"`
	ID   string `json:"id,omitempty"`
}

type OwnerReference struct {
	APIVersion string `json:"apiVersion,omitempty"`
	Kind       string `json:"kind,omitempty"`
	Name       string `json:"name,omitempty"`
}

// RetinaEndpointStatus defines the observed state of RetinaEndpoint
type RetinaEndpointStatus struct{}

//+kubebuilder:object:root=true

// RetinaEndpointList contains a list of RetinaEndpoint
type RetinaEndpointList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RetinaEndpoint `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RetinaEndpoint{}, &RetinaEndpointList{})
}
