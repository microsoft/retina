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

	CaptureNameEnvKey  string = "CAPTURE_NAME"
	NodeHostNameEnvKey string = "NODE_HOST_NAME"

	CaptureFilterEnvKey   string = "CAPTURE_FILTER"
	CaptureDurationEnvKey string = "CAPTURE_DURATION"
	CaptureMaxSizeEnvKey  string = "CAPTURE_MAX_SIZE"
	IncludeMetadataEnvKey string = "INCLUDE_METADATA"
	PacketSizeEnvKey      string = "CAPTURE_PACKET_SIZE"

	TcpdumpFilterEnvKey    string = "TCPDUMP_FILTER"
	TcpdumpRawFilterEnvKey string = "TCPDUMP_RAW_FILTER"
	NetshFilterEnvKey      string = "NETSH_FILTER"

	ApiserverEnvKey = "APISERVER"
)
