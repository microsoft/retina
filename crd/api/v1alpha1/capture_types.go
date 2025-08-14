/*
Copyright (c) Microsoft Corporation.
Licensed under the MIT license.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CaptureConditionType string

const (
	// CaptureComplete indicates the capture is completed.
	CaptureComplete CaptureConditionType = "complete"
	// CaptureError indicates the capture is errored.
	CaptureError CaptureConditionType = "error"
)

// CaptureStatus describes the status of the capture.
type CaptureStatus struct {
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// Represents time when the Capture controller started processing a job.
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty" protobuf:"bytes,2,opt,name=startTime"`

	// Represents time when the Capture was completed, and it is determined by the last completed capture job.
	// The completion time is only set when the Capture finishes successfully.
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty" protobuf:"bytes,3,opt,name=completionTime"`

	// The number of pending and running jobs.
	// +optional
	Active int32 `json:"active,omitempty" protobuf:"varint,4,opt,name=active"`

	// The number of completed jobs.
	// +optional
	Succeeded int32 `json:"succeeded,omitempty" protobuf:"varint,5,opt,name=succeeded"`

	// The number of failed jobs.
	// +optional
	Failed int32 `json:"failed,omitempty" protobuf:"varint,6,opt,name=failed"`
}

// CaptureOption lists the options of the capture.
type CaptureOption struct {
	// Duration indicates length of time that the capture should continue for.
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern="^([0-9]+(\\.[0-9]+)?(ns|us|µs|ms|s|m|h))+$"
	// +optional
	Duration *metav1.Duration `json:"duration,omitempty"`

	// PacketSize limits the each packet to bytes in size and packets longer than PacketSize will be truncated.
	// +optional
	PacketSize *int `json:"packetSize,omitempty"`

	// MaxCaptureSize limits the capture file to MB in size.
	// +kubebuilder:default=100
	// +optional
	MaxCaptureSize *int `json:"maxCaptureSize,omitempty"`

	// Interfaces specifies the network interfaces on which to capture packets.
	// If specified, captures only on the listed interfaces.
	// If empty, captures on all interfaces by default.
	// +optional
	Interfaces []string `json:"interfaces,omitempty"`
}

// CaptureTarget indicates the target on which the network packets capture will be performed.
type CaptureTarget struct {
	// NodeSelector is a selector which select the node to capture network packets.
	// Selector which must match a node's labels.
	// NodeSelector is incompatible with NamespaceSelector/PodSelector pair.
	// +optional
	NodeSelector *metav1.LabelSelector `json:"nodeSelector,omitempty"`

	// NamespaceSelector selects Namespaces using cluster-scoped labels. This field follows
	// standard label selector semantics.
	// NamespaceSelector and PodSelector pair selects a pod to capture pod network namespace traffic.
	// +optional
	NamespaceSelector *metav1.LabelSelector `json:"namespaceSelector,omitempty"`

	// This is a label selector which selects Pods. This field follows standard label
	// selector semantics.
	// +optional
	PodSelector *metav1.LabelSelector `json:"podSelector,omitempty"`
}

// CaptureConfiguration indicates the configurations of the network capture.
type CaptureConfiguration struct {
	CaptureTarget CaptureTarget `json:"captureTarget"`
	// Filters represent a range of filters to be included/excluded in the capture.
	// +optional
	Filters *CaptureConfigurationFilters `json:"filters,omitempty"`

	// TcpdumpFilter is a raw tcpdump filter string.
	// +optional
	TcpdumpFilter *string `json:"tcpdumpFilter,omitempty"`

	// IncludeMetadata represents whether or not networking metadata should be captured.
	// Networking metadata will consists of the following info, but is expected to grow:
	// - IP address configuration
	// - IP neighbor status
	// - IPtables rule dumps
	// - Network statistics information
	// +optional
	// +kubebuilder:default=true
	IncludeMetadata bool `json:"includeMetadata"`

	// +optional
	CaptureOption CaptureOption `json:"captureOption,omitempty"`
}

// CaptureConfigurationFilters presents filters of capture.
type CaptureConfigurationFilters struct {
	// Include specifies what IP or IP:port is included in the capture with wildcard support.
	// If a port not specified or is *, the port filter is excluded.
	// If an IP is specified as *, the host filter should be included.
	// Include and Exclude arguments will finally be translated into a logic like:
	// (include1 or include2) and not (exclude1 or exclude2)
	// +optional
	Include []string `json:"include,omitempty"`
	// Exclude specifies what IP or IP:port is excluded in the capture with wildcard support.
	// See Include for detailed explanation.
	// +optional
	Exclude []string `json:"exclude,omitempty"`
}

// OutputConfiguration indicates the location capture will be stored.
type OutputConfiguration struct {
	// HostPath stores the capture files into the specified host filesystem.
	// If nothing exists at the given path of the host, an empty directory will be created there.
	// +optional
	HostPath *string `json:"hostPath,omitempty"`
	// PersistentVolumeClaim mounts the supplied PVC into the pod on `/capture` and write the capture files there.
	// +optional
	PersistentVolumeClaim *string `json:"persistentVolumeClaim,omitempty"`
	// BlobUpload is a secret containing the blob SAS URL to the given blob container.
	// +optional
	BlobUpload *string `json:"blobUpload,omitempty"`
	// S3Upload configures the details for uploading capture files to an S3-compatible storage service.
	// +optional
	S3Upload *S3Upload `json:"s3Upload,omitempty"`
}

type S3Upload struct {
	// Endpoint of S3 compatible storage service.
	// +optional
	Endpoint string `json:"endpoint,omitempty"`
	// Bucket in which to store the capture.
	// +required
	Bucket string `json:"bucket,omitempty"`
	// SecretName is the name of secret which stores S3 compliant storage access key and secret key.
	// +required
	SecretName string `json:"secretName,omitempty"`
	// Region in which the S3 compatible bucket is located.
	// +optional
	Region string `json:"region,omitempty"`
	// Path specifies the prefix path within the S3 bucket where captures will be stored, e.g., "retina/captures".
	// +optional
	Path string `json:"path,omitempty"`
}

// CaptureSpec indicates the specification of Capture.
type CaptureSpec struct {
	// +kubebuilder:validation:Required
	CaptureConfiguration CaptureConfiguration `json:"captureConfiguration"`
	// +kubebuilder:validation:Required
	OutputConfiguration OutputConfiguration `json:"outputConfiguration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:categories={retina},scope=Namespaced
// +kubebuilder:subresource:status

// Capture indicates the settings of a network trace.
type Capture struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// +kubebuilder:validation:Required
	Spec CaptureSpec `json:"spec"`
	// +optional
	Status CaptureStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CaptureList contains a list of Capture.
type CaptureList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	// +listType=set
	Items []Capture `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Capture{}, &CaptureList{})
}
