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
	cmd := exec.Command("C:\\ContainerPlat\\crictl.exe", "pods", "-q")
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

		fmt.Printf("Inspecting pod ID: %s\n", podID)

		cmd := exec.Command("C:\\ContainerPlat\\crictl.exe", "inspectp", podID)
		var podSpec bytes.Buffer
		cmd.Stdout = &podSpec
		err := cmd.Run()
		if err != nil {
			fmt.Printf("Error running crictl command: %v\n", err)
			return nil, err
		}

		var spec PodSpec
		err = json.Unmarshal(podSpec.Bytes(), &spec)
		if err != nil {
			fmt.Printf("Error unmarshalling JSON: %v\n", err)
			continue
		}

		fmt.Printf("Pod Name: %s, Namespace: %s, IP: %s\n", spec.Status.MetaData.Name, spec.Status.MetaData.Namespace, spec.Status.Network.IP)

		if spec.Status.Network.IP == ip {
			return &cache.PodInfo{
				Name:      spec.Status.MetaData.Name,
				Namespace: spec.Status.MetaData.Namespace,
			}, nil
		}
	}

	return nil, nil
}
