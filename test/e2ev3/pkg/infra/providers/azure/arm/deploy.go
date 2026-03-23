// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package arm

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/microsoft/retina/test/e2ev3/pkg/infra/providers/azure"
)

const (
	deploymentPollFrequency = 30 * time.Second
	deploymentStatusTicker  = 60 * time.Second
)

// DeployInfra is a go-workflow step that generates an ARM template from InfraConfig
// and deploys all e2e infrastructure (resource group, VNet, public IPs, AKS cluster)
// in a single subscription-level ARM deployment.
type DeployInfra struct {
	Config *azure.InfraConfig
}

func (d *DeployInfra) String() string { return "deploy-azure-infra" }

func (d *DeployInfra) Do(ctx context.Context) error {
	log := slog.With("step", d.String())
	template := GenerateTemplate(d.Config)

	templateJSON, err := json.MarshalIndent(template, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal ARM template: %w", err)
	}
	log.Info("generated ARM template", "bytes", len(templateJSON), "cluster", d.Config.ClusterName, "location", d.Config.Location)

	cred, err := azidentity.NewAzureCLICredential(nil)
	if err != nil {
		return fmt.Errorf("failed to obtain Azure CLI credential: %w", err)
	}

	client, err := armresources.NewDeploymentsClient(d.Config.SubscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create deployments client: %w", err)
	}

	deploymentName := fmt.Sprintf("e2e-%s", d.Config.ClusterName)
	log.Info("starting ARM deployment at subscription scope", "deployment", deploymentName)

	poller, err := client.BeginCreateOrUpdateAtSubscriptionScope(ctx, deploymentName, armresources.Deployment{
		Location: to.Ptr(d.Config.Location),
		Properties: &armresources.DeploymentProperties{
			Mode:     to.Ptr(armresources.DeploymentModeIncremental),
			Template: template,
		},
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to begin ARM deployment: %w", err)
	}

	notifychan := make(chan struct{})
	go func() {
		_, err = poller.PollUntilDone(ctx, &runtime.PollUntilDoneOptions{
			Frequency: deploymentPollFrequency,
		})
		close(notifychan)
	}()

	ticker := time.NewTicker(deploymentStatusTicker)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("ARM deployment timed out: %w", ctx.Err())
		case <-ticker.C:
			log.Info("waiting for ARM deployment to complete", "deployment", deploymentName)
		case <-notifychan:
			if err != nil {
				return fmt.Errorf("ARM deployment %q failed: %w", deploymentName, err)
			}
			log.Info("ARM deployment completed successfully", "deployment", deploymentName)
			return nil
		}
	}
}
