// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package provider

import "os"

//go:generate go run go.uber.org/mock/mockgen -source=interface.go -destination=mock_network_capture.go -package=provider Interface
type NetworkCaptureProviderInterface interface {
	// Setup prepares the provider with folder to store network capture for temporary.
	Setup(captureJobName, nodeHostname string) (string, error)
	// CaptureNetworkPacket capture network traffic per user input and store the captured network packets in local directory.
	CaptureNetworkPacket(filter string, duration, maxSize int, sigChan <-chan os.Signal) error
	// CollectMetadata collects network metadata and store network metadata info in local directory.
	CollectMetadata() error
	// Cleanup removes created resources.
	Cleanup() error
}
