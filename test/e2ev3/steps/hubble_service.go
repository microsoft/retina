// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package steps

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	k8s "github.com/microsoft/retina/test/e2ev3/pkg/kubernetes"
)

// ValidateHubbleRelayServiceStep validates that the hubble-relay-service
// exists in the cluster.
type ValidateHubbleRelayServiceStep struct {
	KubeConfigFilePath string
}

func (v *ValidateHubbleRelayServiceStep) Do(ctx context.Context) error {
	step := &k8s.ValidateResource{
		ResourceName:       "hubble-relay-service",
		ResourceNamespace:  k8s.HubbleNamespace,
		ResourceType:       k8s.ResourceTypeService,
		Labels:             "k8s-app=" + k8s.HubbleRelayApp,
		KubeConfigFilePath: v.KubeConfigFilePath,
	}
	return step.Do(ctx)
}

// ValidateHubbleUIServiceStep validates that the hubble-ui service exists
// and that it responds with HTTP 200.
type ValidateHubbleUIServiceStep struct {
	KubeConfigFilePath string
}

func (v *ValidateHubbleUIServiceStep) Do(ctx context.Context) error {
	validateStep := &k8s.ValidateResource{
		ResourceName:       k8s.HubbleUIApp,
		ResourceNamespace:  k8s.HubbleNamespace,
		ResourceType:       k8s.ResourceTypeService,
		Labels:             "k8s-app=" + k8s.HubbleUIApp,
		KubeConfigFilePath: v.KubeConfigFilePath,
	}
	if err := validateStep.Do(ctx); err != nil {
		return fmt.Errorf("failed to validate hubble-ui service: %w", err)
	}

	// Port forward and validate HTTP response
	pf := &k8s.PortForward{
		LabelSelector:         "k8s-app=hubble-ui",
		LocalPort:             "8080",
		RemotePort:            "8081",
		OptionalLabelAffinity: "k8s-app=hubble-ui",
		Endpoint:              "?namespace=default",
		KubeConfigFilePath:    v.KubeConfigFilePath,
	}
	if err := pf.Do(ctx); err != nil {
		return fmt.Errorf("failed to port forward to hubble-ui: %w", err)
	}
	defer pf.Stop() //nolint:errcheck // best effort cleanup

	httpStep := &k8s.ValidateHTTPResponse{
		URL:            "http://localhost:8080",
		ExpectedStatus: http.StatusOK,
	}
	if err := httpStep.Do(ctx); err != nil {
		return fmt.Errorf("failed to validate hubble-ui HTTP response: %w", err)
	}

	log.Printf("Hubble UI service validation succeeded")
	return nil
}

const hubbleUIRequestTimeout = 30 * time.Second

// ValidateHTTPResponseStep wraps the old ValidateHTTPResponse step.
type ValidateHTTPResponseStep struct {
	URL            string
	ExpectedStatus int
}

func (v *ValidateHTTPResponseStep) Do(ctx context.Context) error {
	step := &k8s.ValidateHTTPResponse{
		URL:            v.URL,
		ExpectedStatus: v.ExpectedStatus,
	}
	return step.Do(ctx)
}
