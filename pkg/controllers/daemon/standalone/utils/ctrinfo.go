// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"

	"github.com/microsoft/retina/pkg/common"
)

type CtrinfoSource struct{}

type PodSpec struct {
	Status Status `json:"status"`
}

type Status struct {
	Metadata Metadata   `json:"metadata"`
	Network  PodNetwork `json:"network"`
}

type Metadata struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type PodNetwork struct {
	IP string `json:"ip"`
}

var (
	crictlCommand = runCommand

	errGetPods    = errors.New("failed to get running pods")
	errInspectPod = errors.New("failed to inspect pod information")
	errJSONRead   = errors.New("error unmarshalling JSON")
)

func (cs *CtrinfoSource) GetAllEndpoints() ([]*common.RetinaEndpoint, error) {
	runningPods, err := crictlCommand("cmd", "/c", "crictl", "pods", "-q")
	if err != nil {
		return nil, fmt.Errorf("%v: %v", errGetPods, err)
	}

	podIDs := strings.Split(strings.TrimSpace(runningPods), "\n")
	endpoints := []*common.RetinaEndpoint{}
	for _, podID := range podIDs {
		if podID == "" {
			continue
		}

		podSpec, err := crictlCommand("cmd", "/c", "crictl", "inspectp", podID)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", errInspectPod, err)
		}

		var spec PodSpec
		if err := json.Unmarshal([]byte(podSpec), &spec); err != nil {
			return nil, fmt.Errorf("%w: %v", errJSONRead, err)
		}

		ip := net.ParseIP(spec.Status.Network.IP)
		if ip == nil {
			// Skip pods with invalid or empty IPs
			continue
		}

		endpoints = append(endpoints, common.NewRetinaEndpoint(
			spec.Status.Metadata.Name,
			spec.Status.Metadata.Namespace,
			common.NewIPAddress(ip, nil),
		))
	}

	return endpoints, nil
}

func runCommand(command string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, command, args...)
	var output bytes.Buffer
	cmd.Stdout = &output
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("failed to run command: %w", err)
	}
	return output.String(), nil
}
