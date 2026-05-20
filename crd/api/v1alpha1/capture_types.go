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
	// If specified, captures only on the listed interfaces (e.g., ["eth0", "eth1"]).
	// If empty, captures on all interfaces by default.
	// Use this field to select specific interfaces, NOT the tcpdumpFilter field.
	// +optional
	Interfaces []string `json:"interfaces,omitempty"`

	// PcapFilter specifies a BPF filter expression for packet filtering (e.g., "tcp port 443", "host 10.0.0.1").
	// Only BPF expressions are allowed, no flags. See https://www.tcpdump.org/manpages/pcap-filter.7.html
	// +optional
	// +kubebuilder:validation:MaxLength=1024
	// +kubebuilder:validation:Pattern="^[^-]*$"
	PcapFilter *string `json:"pcapFilter,omitempty"`

	// NoPromiscuous disables promiscuous mode for packet capture.
	// When true, only packets destined for this host are captured (equivalent to tcpdump -p flag).
	// When false or unset, captures all packets on the network segment (default behavior).
	// +optional
	NoPromiscuous *bool `json:"noPromiscuous,omitempty"`

	// PacketBuffered enables packet-buffered output mode (equivalent to tcpdump -U flag).
	// When true, packets are written to output as soon as they're captured rather than being buffered.
	// Useful for real-time monitoring but may impact performance.
	// +optional
	PacketBuffered *bool `json:"packetBuffered,omitempty"`

	// ImmediateMode enables immediate mode for packet capture (equivalent to tcpdump --immediate-mode).
	// When true, packets are delivered to the application immediately rather than being buffered.
	// This can reduce latency but may increase CPU usage.
	// +optional
	ImmediateMode *bool `json:"immediateMode,omitempty"`

	// NoResolveDNS disables DNS resolution for captured addresses (equivalent to tcpdump -n flag).
	// When true, IP addresses are displayed numerically without resolving hostnames.
	// This speeds up capture processing and avoids DNS lookup overhead.
	// +optional
	NoResolveDNS *bool `json:"noResolveDNS,omitempty"`

	// NoResolvePort disables port name resolution (equivalent to tcpdump -nn flag).
	// When true, both IP addresses and port numbers are displayed numerically.
	// This prevents service name lookups for port numbers.
	// +optional
	NoResolvePort *bool `json:"noResolvePort,omitempty"`

	// Verbosity controls the verbosity level of packet capture output.
	// Valid values: "" (normal/default), "verbose" (tcpdump -v), "extra" (tcpdump -vv), "max" (tcpdump -vvv).
	// Empty string means normal verbosity with no extra verbose flags.
	// +optional
	// +kubebuilder:validation:Enum="";verbose;extra;max
	Verbosity *string `json:"verbosity,omitempty"`

	// PrintDataFormat controls how packet data is printed in the output.
	// Valid values: "" (none), "hex" (tcpdump -x), "hex-with-link" (tcpdump -xx), "ascii" (tcpdump -A), "ascii-with-link" (tcpdump -AA).
	// Empty string means no packet data printing.
	// +optional
	// +kubebuilder:validation:Enum="";hex;hex-with-link;ascii;ascii-with-link
	PrintDataFormat *string `json:"printDataFormat,omitempty"`

	// PrintLinkHeader prints link-level (Ethernet) headers (equivalent to tcpdump -e flag).
	// Shows MAC addresses and other link-layer information.
	// +optional
	PrintLinkHeader *bool `json:"printLinkHeader,omitempty"`

	// QuietOutput enables quiet/quick output mode (equivalent to tcpdump -q flag).
	// Prints less protocol information for shorter output lines.
	// +optional
	QuietOutput *bool `json:"quietOutput,omitempty"`

	// AbsoluteSeq prints absolute TCP sequence numbers (equivalent to tcpdump -S flag).
	// Shows actual sequence numbers instead of relative numbers.
	// +optional
	AbsoluteSeq *bool `json:"absoluteSeq,omitempty"`

	// TimestampFormat controls the timestamp format in packet capture output.
	// Valid values: "" (default), "none" (tcpdump -t), "unformatted" (tcpdump -tt), "delta" (tcpdump -ttt), "date" (tcpdump -tttt), "delta-since-first" (tcpdump -ttttt).
	// Empty string means default timestamp format.
	// +optional
	// +kubebuilder:validation:Enum="";none;unformatted;delta;date;delta-since-first
	TimestampFormat *string `json:"timestampFormat,omitempty"`

	// DontVerifyChecksum disables TCP checksum verification (equivalent to tcpdump -K flag).
	// Skips TCP checksum validation for captured packets.
	// +optional
	DontVerifyChecksum *bool `json:"dontVerifyChecksum,omitempty"`
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

	// PodNames allows selecting specific pods by their names.
	// If specified, the capture will be performed on the pods with matching names in the specified namespace.
	// PodNames is incompatible with NodeSelector, NamespaceSelector, and PodSelector.
	// +optional
	PodNames []string `json:"podNames,omitempty"`
}

// CaptureConfiguration indicates the configurations of the network capture.
type CaptureConfiguration struct {
	CaptureTarget CaptureTarget `json:"captureTarget"`
	// Filters represent a range of filters to be included/excluded in the capture.
	// +optional
	Filters *CaptureConfigurationFilters `json:"filters,omitempty"`

	// TcpdumpFilter accepts BPF filter expressions only (no flags).
	//
	// Deprecated: Use captureOption.pcapFilter for BPF expressions and captureOption boolean flags for display options.
	// +optional
	// +kubebuilder:validation:MaxLength=1024
	// +kubebuilder:validation:Pattern="^[^-]*$"
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
