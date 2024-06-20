package conntrack

import "github.com/microsoft/retina/pkg/plugin/api"

const (
	Name api.PluginName = "conntrack"
	TCP  uint8          = 6  // Transmission Control Protocol
	UDP  uint8          = 17 // User Datagram Protocol
	// Hardcoded pod CIDR and service CIDR for now, we should be getting this via the pod's environment variables
	PodCIDR     string = "192.168.0.0/16"
	ServiceCIDR string = "10.0.0.0/16"
)
