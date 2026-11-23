// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package kubernetes

import (
	"fmt"
	"log"
	"os"
	"time"

	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/cli"
	helmValues "helm.sh/helm/v4/pkg/cli/values"
	"helm.sh/helm/v4/pkg/getter"
	"helm.sh/helm/v4/pkg/kube"
	releasev1 "helm.sh/helm/v4/pkg/release/v1"
)

const upgradeTimeout = 300 * time.Second // longer timeout to accommodate slow windows node terminating and restarting.

type UpgradeRetinaHelmChart struct {
	Namespace          string
	ReleaseName        string
	KubeConfigFilePath string
	ChartPath          string
	TagEnv             string
	ValuesFile         string
}

func (u *UpgradeRetinaHelmChart) Run() error {
	settings := cli.New()
	settings.KubeConfig = u.KubeConfigFilePath
	actionConfig := new(action.Configuration)

	err := actionConfig.Init(settings.RESTClientGetter(), u.Namespace, os.Getenv("HELM_DRIVER"))
	if err != nil {
		return fmt.Errorf("failed to initialize helm action config: %w", err)
	}

	client := action.NewUpgrade(actionConfig)
	client.WaitStrategy = kube.StatusWatcherStrategy
	client.WaitForJobs = true
	client.Timeout = upgradeTimeout
	// Reuse the values stored on the release so the install-time overrides
	// (image registry, repository, tag) survive the upgrade; without this the
	// chart-default images are restored and the agent reverts to an old tag.
	client.ReuseValues = true

	// Create a new Get action
	get := action.NewGet(actionConfig)

	// Get the current release
	rel, err := get.Run(u.ReleaseName)
	if err != nil {
		return fmt.Errorf("failed to get release: %w", err)
	}
	current, ok := rel.(*releasev1.Release)
	if !ok {
		return fmt.Errorf("%w: %T", errUnexpectedReleaseType, rel)
	}

	// Get the chart from the current release
	chart := current.Chart

	// enable advanced metrics profile
	options := helmValues.Options{
		ValueFiles: []string{u.ValuesFile},
	}
	provider := getter.All(settings)
	values, err := options.MergeValues(provider)
	if err != nil {
		return fmt.Errorf("failed to merge values: %w", err)
	}
	// logs values to be set during upgrade
	log.Printf("values to be set during upgrade: %v\n", values)

	rel, err = client.Run(u.ReleaseName, chart, values)
	if err != nil {
		return fmt.Errorf("failed to upgrade chart: %w", err)
	}
	upgraded, ok := rel.(*releasev1.Release)
	if !ok {
		return fmt.Errorf("%w: %T", errUnexpectedReleaseType, rel)
	}

	log.Printf("upgraded chart from path: %s in namespace: %s\n", upgraded.Name, upgraded.Namespace)
	// this will confirm the values set during installation
	log.Printf("chart values: %v\n", upgraded.Config)

	return nil
}

func (u *UpgradeRetinaHelmChart) Prevalidate() error {
	return nil
}

func (u *UpgradeRetinaHelmChart) Stop() error {
	return nil
}
