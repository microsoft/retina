// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package steps

import (
	"github.com/microsoft/retina/test/e2ev3/framework/constants"
	"github.com/microsoft/retina/test/e2ev3/common"
)

// Hubble TCP label maps used with common.ValidateMetricStep.
var (
	HubbleTCPPodName = "agnhost-tcp-0"

	ValidHubbleTCPSYNFlag = map[string]string{
		constants.HubbleSourceLabel:      common.TestPodNamespace + "/" + HubbleTCPPodName,
		constants.HubbleDestinationLabel: "",
		constants.HubbleFamilyLabel:      constants.IPV4,
		constants.HubbleFlagLabel:        constants.SYN,
	}

	ValidHubbleTCPFINFlag = map[string]string{
		constants.HubbleSourceLabel:      common.TestPodNamespace + "/" + HubbleTCPPodName,
		constants.HubbleDestinationLabel: "",
		constants.HubbleFamilyLabel:      constants.IPV4,
		constants.HubbleFlagLabel:        constants.FIN,
	}

	ValidHubbleTCPMetricsLabels = []map[string]string{
		ValidHubbleTCPSYNFlag,
		ValidHubbleTCPFINFlag,
	}
)
