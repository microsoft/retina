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

type PodSpec struct {
	Status Status `json:"status"`
}

type Status struct {
	Metadata Metadata `json:"metadata"`
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
	cmd := exec.Command("Y:\\__SfCriContainerd\\crictl.exe", "pods", "-q") // investigate in
	// cmd := exec.Command("C:\\ContainerPlat\\crictl.exe", "pods", "-q") // investigate in ACI

	var output bytes.Buffer
	cmd.Stdout = &output
	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("Failed to get running pods: %w", err)
	}

	podIDs := strings.SplitSeq(strings.TrimSpace(output.String()), "\n")

	for podID := range podIDs {
		if podID == "" {
			continue
		}

		fmt.Printf("Inspecting pod ID: %s\n", podID)

		cmd := exec.Command("Y:\\__SfCriContainerd\\crictl.exe", "inspectp", podID) // investigate in ACI
		// cmd := exec.Command("C:\\ContainerPlat\\crictl.exe", "inspectp", podID) // investigate in ACI

		var podSpec bytes.Buffer
		cmd.Stdout = &podSpec
		err := cmd.Run()
		if err != nil {
			return nil, fmt.Errorf("Failed to inspect pod information: %w", err)
		}

		var spec PodSpec
		err = json.Unmarshal(podSpec.Bytes(), &spec)
		if err != nil {
			fmt.Printf("Error unmarshalling JSON: %v\n", err)
			continue
		}

		if spec.Status.Network.IP == ip {
			return &cache.PodInfo{
				Name:      spec.Status.Metadata.Name,
				Namespace: spec.Status.Metadata.Namespace,
			}, nil
		}
	}

	// fmt.Printf("IP address %s not found in containerd\n", ip)
	return nil, nil
}
