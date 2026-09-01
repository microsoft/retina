// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package ebpfwindows

import (
	"testing"

	flow "github.com/cilium/cilium/api/v1/flow"
	"github.com/microsoft/retina/pkg/utils"
	"github.com/stretchr/testify/require"
)

func TestNormalizeFlowNil(t *testing.T) {
	require.Nil(t, normalizeFlow(nil))
}

func TestNormalizeFlowForwardedDefaults(t *testing.T) {
	fl := &flow.Flow{IP: &flow.IP{Source: "10.0.0.1", Destination: "10.0.0.2"}}
	out := normalizeFlow(fl)
	require.NotNil(t, out)
	require.Equal(t, flow.Verdict_FORWARDED, out.GetVerdict())
	require.Equal(t, flow.TrafficDirection_INGRESS, out.GetTrafficDirection())
}

func TestNormalizeFlowForwardedPreservesDirection(t *testing.T) {
	fl := &flow.Flow{
		Verdict:          flow.Verdict_FORWARDED,
		TrafficDirection: flow.TrafficDirection_EGRESS,
		IP:               &flow.IP{Source: "10.0.0.1", Destination: "10.0.0.2"},
	}
	out := normalizeFlow(fl)
	require.Equal(t, flow.TrafficDirection_EGRESS, out.GetTrafficDirection())
	require.Equal(t, flow.Verdict_FORWARDED, out.GetVerdict())
}

func TestNormalizeFlowDerivesDirectionFromObservationPoint(t *testing.T) {
	fl := &flow.Flow{
		Verdict:               flow.Verdict_FORWARDED,
		TraceObservationPoint: flow.TraceObservationPoint_TO_NETWORK,
		IP:                    &flow.IP{Source: "10.0.0.1", Destination: "10.0.0.2"},
	}
	out := normalizeFlow(fl)
	require.Equal(t, flow.TrafficDirection_EGRESS, out.GetTrafficDirection())

	fl2 := &flow.Flow{
		Verdict:               flow.Verdict_FORWARDED,
		TraceObservationPoint: flow.TraceObservationPoint_TO_ENDPOINT,
	}
	require.Equal(t, flow.TrafficDirection_INGRESS, normalizeFlow(fl2).GetTrafficDirection())
}

func TestNormalizeFlowDropAddsReasonExtension(t *testing.T) {
	fl := &flow.Flow{
		Verdict:          flow.Verdict_DROPPED,
		TrafficDirection: flow.TrafficDirection_INGRESS,
		DropReason:       uint32(flow.DropReason_POLICY_DENIED),
		IP:               &flow.IP{Source: "10.0.0.1", Destination: "10.0.0.2"},
	}
	out := normalizeFlow(fl)
	require.Equal(t, flow.Verdict_DROPPED, out.GetVerdict())
	require.NotEmpty(t, utils.DropReasonDescription(out), "expected a drop_reason extension to be set")
}

func TestNormalizeFlowDropReasonFromDesc(t *testing.T) {
	// Drops are gated on Verdict == DROPPED; the reason label is taken from
	// DropReasonDesc, so a drop with an explicit desc gets a non-empty reason.
	fl := &flow.Flow{
		Verdict:        flow.Verdict_DROPPED,
		DropReasonDesc: flow.DropReason_POLICY_DENIED,
		IP:             &flow.IP{Source: "10.0.0.1", Destination: "10.0.0.2"},
	}
	out := normalizeFlow(fl)
	require.Equal(t, flow.Verdict_DROPPED, out.GetVerdict())
	require.NotEmpty(t, utils.DropReasonDescription(out))
}
