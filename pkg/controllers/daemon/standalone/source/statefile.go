// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package source

import (
	"encoding/json"
	"fmt"
	"net"
	"os"

	"github.com/microsoft/retina/pkg/common"
)

type Statefile struct{}

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

func (s *Statefile) GetAllEndpoints() ([]*common.RetinaEndpoint, error) {
	data, err := os.ReadFile(StateFileLocation)
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

	endpoints := []*common.RetinaEndpoint{}

	// For every HNS endpoint, we check if the equivalent IP address exists in the CNI state file
	for _, iface := range cniState.Network.ExternalInterfaces {
		for _, networkInfo := range iface.Networks {
			for _, endpoint := range networkInfo.Endpoints {
				for _, ipInfo := range endpoint.IPAddresses {
					ip := ipInfo.IP
					if ip == "" {
						continue
					}

					endpoints = append(endpoints, common.NewRetinaEndpoint(
						endpoint.PodName,
						endpoint.PodNamespace,
						common.NewIPAddress(net.ParseIP(ip), nil),
					))
				}
			}
		}
	}

	return endpoints, nil
}
