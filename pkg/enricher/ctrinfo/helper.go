// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package ctrinfo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/microsoft/retina/pkg/controllers/cache"
)

type Status struct {
	MetaData Metadata `json:"metadata"`
	Network  Network  `json:"network"`
}

type Metadata struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type Network struct {
	IP string `json:"ip"`
}

func GetPodInfo(ip string) (*cache.PodInfo, error) {
	cmd := exec.Command("crictl", "pods", "-q")
	var output bytes.Buffer

	cmd.Stdout = &output
	err := cmd.Run()
	if err != nil {
		fmt.Printf("Error running crictl command: %v\n", err)
		return nil, err
	}

	// Gather all pod IDs
	podIDs := strings.Split(strings.TrimSpace(output.String()), "\n")

	for _, podID := range podIDs {
		if podID == "" {
			continue
		}

		cmd := exec.Command("crictl", "inspectp", podID)
		var podSpec bytes.Buffer
		cmd.Stdout = &podSpec
		err := cmd.Run()
		if err != nil {
			fmt.Printf("Error running crictl command: %v\n", err)
			return nil, err
		}

		var podStatus Status
		err = json.Unmarshal(podSpec.Bytes(), &podStatus)
		if err != nil {
			fmt.Printf("Error unmarshalling JSON: %v\n", err)
			continue
		}

		if podStatus.Network.IP == ip {
			return &cache.PodInfo{
				Name:      podStatus.MetaData.Name,
				Namespace: podStatus.MetaData.Namespace,
			}, nil
		}
	}

	return nil, nil
}
