// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package statefile

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/microsoft/retina/pkg/controllers/cache"
)

var StateFileLocation = "C:/Windows/System32/azure-vnet.json"

type CniState struct {
	Network Network `json:"Network"`
}

type Network struct {
	ExternalInterfaces map[string]ExternalInterface `json:"ExternalInterfaces"`
}

type ExternalInterface struct {
	Networks map[string]NetworkInfo `json:"Networks"`
}

type NetworkInfo struct {
	Endpoints map[string]Endpoint `json:"Endpoints"`
}

type Endpoint struct {
	ID           string   `json:"Id"`
	IPAddresses  []IPInfo `json:"IPAddresses"`
	PodName      string   `json:"PodName"`
	PodNamespace string   `json:"PodNamespace"`
}

type IPInfo struct {
	IP string `json:"IP"`
}

func GetPodInfo(ip, filePath string) (*cache.PodInfo, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CNI state file: %w", err)
	}

	if len(data) == 0 {
		return nil, nil
	}

	var cniState CniState
	if err := json.Unmarshal(data, &cniState); err != nil {
		return nil, fmt.Errorf("failed to decode CNI state file: %w", err)
	}

	// For every HNS endpoint, we check if the equivalent IP address exists in the CNI state file
	for _, iface := range cniState.Network.ExternalInterfaces {
		for _, networkInfo := range iface.Networks {
			for _, endpoint := range networkInfo.Endpoints {
				for _, ipInfo := range endpoint.IPAddresses {
					if ipInfo.IP == ip {
						return &cache.PodInfo{
							Name:      endpoint.PodName,
							Namespace: endpoint.PodNamespace,
						}, nil
					}
				}
			}
		}
	}

	return nil, nil
}
