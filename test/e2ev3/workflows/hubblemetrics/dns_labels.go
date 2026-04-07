// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package hubblemetrics

import (
	"github.com/microsoft/retina/test/e2ev3/config"
)

// Hubble DNS test fixtures: pod name and expected metric labels.
var HubbleDNSPodName = "agnhost-dns-0"

func ValidHubbleDNSQueryMetricLabels(namespace string) map[string]string {
	return map[string]string{
		config.HubbleDestinationLabel: "",
		config.HubbleSourceLabel:      namespace + "/" + HubbleDNSPodName,
		config.HubbleIPsRetunedLabel:  "0",
		config.HubbleQTypesLabel:      "A",
		config.HubbleRCodeLabel:       "",
		config.HubbleQueryLabel:       "one.one.one.one.",
	}
}

func ValidHubbleDNSResponseMetricLabels(namespace string) map[string]string {
	return map[string]string{
		config.HubbleDestinationLabel: namespace + "/" + HubbleDNSPodName,
		config.HubbleSourceLabel:      "",
		config.HubbleIPsRetunedLabel:  "2",
		config.HubbleQTypesLabel:      "A",
		config.HubbleRCodeLabel:       "No Error",
		config.HubbleQueryLabel:       "one.one.one.one.",
	}
}
