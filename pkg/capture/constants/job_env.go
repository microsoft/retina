// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package constants

type (
	CaptureOutputLocationEnvKey string
)

// Environment variables
const (
	CaptureOutputLocationEnvKeyHostPath              CaptureOutputLocationEnvKey = "HOSTPATH"
	CaptureOutputLocationEnvKeyPersistentVolumeClaim CaptureOutputLocationEnvKey = "PERSISTENT_VOLUME_CLAIM"
	CaptureOutputLocationEnvKeyS3Endpoint            CaptureOutputLocationEnvKey = "S3_ENDPOINT"
	CaptureOutputLocationEnvKeyS3Region              CaptureOutputLocationEnvKey = "S3_REGION"
	CaptureOutputLocationEnvKeyS3Bucket              CaptureOutputLocationEnvKey = "S3_BUCKET"
	CaptureOutputLocationEnvKeyS3Path                CaptureOutputLocationEnvKey = "S3_PATH"

	CaptureNameEnvKey           string = "CAPTURE_NAME"
	NodeHostNameEnvKey          string = "NODE_HOST_NAME"
	CaptureStartTimestampEnvKey string = "CAPTURE_START_TIMESTAMP"

	NamespaceEnvKey     string = "NAMESPACE"
	PodNameEnvKey       string = "POD_NAME"
	ContainerNameEnvKey string = "CONTAINER_NAME"

	CaptureFilterEnvKey   string = "CAPTURE_FILTER"
	CaptureDurationEnvKey string = "CAPTURE_DURATION"
	CaptureMaxSizeEnvKey  string = "CAPTURE_MAX_SIZE"
	IncludeMetadataEnvKey string = "INCLUDE_METADATA"
	PacketSizeEnvKey      string = "CAPTURE_PACKET_SIZE"

	TcpdumpFilterEnvKey    string = "TCPDUMP_FILTER"
	TcpdumpRawFilterEnvKey string = "TCPDUMP_RAW_FILTER"
	NetshFilterEnvKey      string = "NETSH_FILTER"

	// Interface selection environment variables
	CaptureInterfacesEnvKey string = "CAPTURE_INTERFACES"

	ApiserverEnvKey = "APISERVER"
)
