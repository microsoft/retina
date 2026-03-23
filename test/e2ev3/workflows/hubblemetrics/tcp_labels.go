// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package hubblemetrics

import (
	"github.com/microsoft/retina/test/e2ev3/config"
)

// Hubble TCP test fixtures: pod name and expected metric labels.
var (
	HubbleTCPPodName = "agnhost-tcp-0"

	ValidHubbleTCPSYNFlag = map[string]string{
		config.HubbleSourceLabel:      config.TestPodNamespace + "/" + HubbleTCPPodName,
		config.HubbleDestinationLabel: "",
		config.HubbleFamilyLabel:      config.IPV4,
		config.HubbleFlagLabel:        config.SYN,
	}

	ValidHubbleTCPFINFlag = map[string]string{
		config.HubbleSourceLabel:      config.TestPodNamespace + "/" + HubbleTCPPodName,
		config.HubbleDestinationLabel: "",
		config.HubbleFamilyLabel:      config.IPV4,
		config.HubbleFlagLabel:        config.FIN,
	}

	ValidHubbleTCPMetricsLabels = []map[string]string{
		ValidHubbleTCPSYNFlag,
		ValidHubbleTCPFINFlag,
	}
)
