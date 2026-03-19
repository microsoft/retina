// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package steps

import (
	"github.com/microsoft/retina/test/e2ev3/pkg/config"
	"github.com/microsoft/retina/test/e2ev3/common"
)

// Hubble DNS label maps used with common.ValidateMetricStep.
var (
	HubbleDNSPodName = "agnhost-dns-0"

	ValidHubbleDNSQueryMetricLabels = map[string]string{
		config.HubbleDestinationLabel: "",
		config.HubbleSourceLabel:      common.TestPodNamespace + "/" + HubbleDNSPodName,
		config.HubbleIPsRetunedLabel:  "0",
		config.HubbleQTypesLabel:      "A",
		config.HubbleRCodeLabel:       "",
		config.HubbleQueryLabel:       "one.one.one.one.",
	}

	ValidHubbleDNSResponseMetricLabels = map[string]string{
		config.HubbleDestinationLabel: common.TestPodNamespace + "/" + HubbleDNSPodName,
		config.HubbleSourceLabel:      "",
		config.HubbleIPsRetunedLabel:  "2",
		config.HubbleQTypesLabel:      "A",
		config.HubbleRCodeLabel:       "No Error",
		config.HubbleQueryLabel:       "one.one.one.one.",
	}
)
