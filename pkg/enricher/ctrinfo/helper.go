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

var crictlCommand = runCommand

func GetPodInfo(ip string) (*cache.PodInfo, error) {
	runningPods, err := crictlCommand("cmd", "/c", "crictl", "pods", "-q")
	if err != nil {
		return nil, fmt.Errorf("Failed to get running pods: %w", err)
	}

	podIDs := strings.Split(strings.TrimSpace(runningPods), "\n")
	for _, podID := range podIDs {
		if podID == "" {
			continue
		}

		podSpec, err := crictlCommand("cmd", "/c", "crictl", "inspectp", podID)
		if err != nil {
			return nil, fmt.Errorf("Failed to inspect pod information: %w", err)
		}

		var spec PodSpec
		if err := json.Unmarshal([]byte(podSpec), &spec); err != nil {
			return nil, fmt.Errorf("Error unmarshalling JSON: %w", err)
		}

		if spec.Status.Network.IP == ip {
			return &cache.PodInfo{
				Name:      spec.Status.Metadata.Name,
				Namespace: spec.Status.Metadata.Namespace,
			}, nil
		}
	}

	return nil, nil
}

func runCommand(command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)
	var output bytes.Buffer
	cmd.Stdout = &output
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("Failed to run command: %w", err)
	}
	return output.String(), nil
}
