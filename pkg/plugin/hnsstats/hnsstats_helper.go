// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package hnsstats

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/microsoft/retina/pkg/controllers/cache"
)

const (
	state_file_location = "C:/k/azure-vnet.json"
)

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
	Id           string   `json:"Id"`
	IpAddresses  []IPInfo `json:"IpAddresses"`
	PodName      string   `json:"PodName"`
	PodNamespace string   `json:"PodNamespace"`
}

type IPInfo struct {
	Ip string `json:"IP"`
}

func GetPodInfo(ip, filePath string) (*cache.PodInfo, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Println("Error reading file:", err)
		return nil, fmt.Errorf("failed to read CNI state file: %w", err)
	}

	var cniState CniState
	if err := json.Unmarshal(data, &cniState); err != nil {
		fmt.Println("Error decoding file:", err)
		return nil, fmt.Errorf("failed to decode CNI state file: %w", err)
	}

	// For every HNS endpoint, we check if the equivalent IP address exists in the CNI state file
	for _, iface := range cniState.Network.ExternalInterfaces {
		for _, networkInfo := range iface.Networks {
			for _, endpoint := range networkInfo.Endpoints {
				for _, ipInfo := range endpoint.IpAddresses {
					if ipInfo.Ip == ip {
						return &cache.PodInfo{
							Name:      endpoint.PodName,
							Namespace: endpoint.PodNamespace,
						}, nil
					}
				}
			}
		}
	}

	return nil, fmt.Errorf("IP address %s not found in CNI state file", ip)
}
