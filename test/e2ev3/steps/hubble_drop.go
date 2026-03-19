// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build e2e

package steps

import (
	"github.com/microsoft/retina/test/e2ev3/pkg/config"
)

// Hubble drop label maps used with common.ValidateMetricStep.
var (
	HubbleDropPodName     = "agnhost-drop-0"
	HubbleDropAgnhostName = "agnhost-drop"

	ValidRetinaDropMetricLabels = map[string]string{
		config.RetinaReasonLabel:    config.IPTableRuleDrop,
		config.RetinaDirectionLabel: "unknown",
	}

	// Note: When the agnhost pod (with deny-all network policy) tries to curl bing.com,
	// it triggers a DNS lookup to CoreDNS. The network policy blocks this egress traffic,
	// but Cilium/Hubble records the drop at the destination (CoreDNS) ingress side rather
	// than the source (agnhost) egress side.
	// We partially validate this metric.
	ValidHubbleDropMetricLabels = map[string]string{
		config.HubbleSourceLabel:   "",
		config.HubbleProtocolLabel: config.UDP,
		config.HubbleReasonLabel:   "POLICY_DENIED",
	}
)
