// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package steps

import (
	"github.com/microsoft/retina/test/e2ev3/pkg/config"
	"github.com/microsoft/retina/test/e2ev3/common"
)

// Hubble TCP label maps used with common.ValidateMetricStep.
var (
	HubbleTCPPodName = "agnhost-tcp-0"

	ValidHubbleTCPSYNFlag = map[string]string{
		config.HubbleSourceLabel:      common.TestPodNamespace + "/" + HubbleTCPPodName,
		config.HubbleDestinationLabel: "",
		config.HubbleFamilyLabel:      config.IPV4,
		config.HubbleFlagLabel:        config.SYN,
	}

	ValidHubbleTCPFINFlag = map[string]string{
		config.HubbleSourceLabel:      common.TestPodNamespace + "/" + HubbleTCPPodName,
		config.HubbleDestinationLabel: "",
		config.HubbleFamilyLabel:      config.IPV4,
		config.HubbleFlagLabel:        config.FIN,
	}

	ValidHubbleTCPMetricsLabels = []map[string]string{
		ValidHubbleTCPSYNFlag,
		ValidHubbleTCPFINFlag,
	}
)
