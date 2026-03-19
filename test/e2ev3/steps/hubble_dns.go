// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package steps

import (
	"github.com/microsoft/retina/test/e2ev3/framework/constants"
	"github.com/microsoft/retina/test/e2ev3/common"
)

// Hubble DNS label maps used with common.ValidateMetricStep.
var (
	HubbleDNSPodName = "agnhost-dns-0"

	ValidHubbleDNSQueryMetricLabels = map[string]string{
		constants.HubbleDestinationLabel: "",
		constants.HubbleSourceLabel:      common.TestPodNamespace + "/" + HubbleDNSPodName,
		constants.HubbleIPsRetunedLabel:  "0",
		constants.HubbleQTypesLabel:      "A",
		constants.HubbleRCodeLabel:       "",
		constants.HubbleQueryLabel:       "one.one.one.one.",
	}

	ValidHubbleDNSResponseMetricLabels = map[string]string{
		constants.HubbleDestinationLabel: common.TestPodNamespace + "/" + HubbleDNSPodName,
		constants.HubbleSourceLabel:      "",
		constants.HubbleIPsRetunedLabel:  "2",
		constants.HubbleQTypesLabel:      "A",
		constants.HubbleRCodeLabel:       "No Error",
		constants.HubbleQueryLabel:       "one.one.one.one.",
	}
)
