// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package ebpfwindows

import (
	flow "github.com/cilium/cilium/api/v1/flow"
	"github.com/microsoft/retina/pkg/utils"
)

// normalizeFlow maps a WCN / eBPF-for-Windows flow onto the fields that Retina's
// flow metrics decoders read (pkg/module/metrics). Retina's drop and forward
// metrics are gated on fl.Verdict and labelled by TrafficDirection; the drop
// metric reads the drop reason from the flow's "drop_reason" extension. The WCN
// producer may leave some of these unset, so we normalize them here so the
// highest-value eBPF telemetry (verdict, drop reason, direction) is surfaced.
func normalizeFlow(fl *flow.Flow) *flow.Flow {
	if fl == nil {
		return nil
	}

	// Direction drives the Forward/Drop metric labels. Derive a direction from
	// the trace observation point when the producer did not set one.
	if fl.GetTrafficDirection() == flow.TrafficDirection_TRAFFIC_DIRECTION_UNKNOWN {
		fl.TrafficDirection = directionFromObservationPoint(fl.GetTraceObservationPoint())
	}

	// Drops (Verdict == DROPPED): surface a drop reason in the flow's extensions
	// so the drop-count metric sees a real reason instead of Unknown.
	// Drops: Retina's drop-count metric is gated on Verdict == DROPPED, so that
	// (not a stray reason enum) is what decides the drop path.
	if fl.GetVerdict() == flow.Verdict_DROPPED {
		ensureDropReason(fl)
		return fl
	}

	// Forward: default an unset verdict to FORWARDED so forward metrics count it.
	if fl.GetVerdict() == flow.Verdict_VERDICT_UNKNOWN {
		fl.Verdict = flow.Verdict_FORWARDED
		fl.TrafficDirection = normalizeDirection(fl.GetTrafficDirection())
	}
	return fl
}

// directionFromObservationPoint maps a trace observation point to a traffic
// direction, mirroring utils.ToFlow for flows whose producer left direction unset.
func directionFromObservationPoint(pt flow.TraceObservationPoint) flow.TrafficDirection {
	switch pt { //nolint:exhaustive // unknown and non-L3/L4 points map to UNKNOWN via default
	case flow.TraceObservationPoint_TO_STACK, flow.TraceObservationPoint_TO_NETWORK:
		return flow.TrafficDirection_EGRESS
	case flow.TraceObservationPoint_TO_ENDPOINT, flow.TraceObservationPoint_FROM_NETWORK:
		return flow.TrafficDirection_INGRESS
	default:
		return flow.TrafficDirection_TRAFFIC_DIRECTION_UNKNOWN
	}
}

// normalizeDirection ensures an unknown direction defaults to INGRESS, which is
// the direction the forward metric labels expect when nothing else is known.
func normalizeDirection(d flow.TrafficDirection) flow.TrafficDirection {
	if d == flow.TrafficDirection_TRAFFIC_DIRECTION_UNKNOWN {
		return flow.TrafficDirection_INGRESS
	}
	return d
}

// ensureDropReason sets the DROPPED verdict and writes the producer's drop
// reason into the flow's "drop_reason" extension so Retina's drop metric is
// labelled with a real reason rather than "Unknown".
func ensureDropReason(fl *flow.Flow) {
	fl.Verdict = flow.Verdict_DROPPED

	ext := utils.GetExtensionsStruct(fl)
	if ext == nil {
		ext = utils.NewExtensions()
	}

	// The producer's Cilium drop reason maps to the Retina DropReason enum that
	// drives DropReasonDescription. Populate the extension BEFORE attaching it to
	// the flow: SetExtensions is a no-op on an empty struct, and AddDropReason is
	// what fills the drop_reason field the metric label reads.
	utils.AddDropReason(fl, ext, dropReasonToUint16(dropReasonFromFlow(fl.GetDropReasonDesc())))
	utils.SetExtensions(fl, ext)
}

// dropReasonFromFlow maps the Cilium flow drop-reason index (uint32) onto
// Retina's Windows-oriented DropReason enum, defaulting to a recognizable
// sentinel so the drop remains visible.
func dropReasonFromFlow(r flow.DropReason) utils.DropReason {
	switch r { //nolint:exhaustive // default covers the remaining proto reasons
	case flow.DropReason_POLICY_DENIED, flow.DropReason_POLICY_DENY:
		return utils.DropReason_Drop_FailedSecurityPolicy
	case flow.DropReason_CT_NO_MAP_FOUND, flow.DropReason_SNAT_NO_MAP_FOUND, flow.DropReason_NO_MAPPING_FOR_NAT_MASQUERADE:
		return utils.DropReason_Drop_InvalidConfig
	case flow.DropReason_UNKNOWN_CONNECTION_TRACKING_STATE:
		return utils.DropReason_Drop_StormLimit
	case flow.DropReason_DROP_REASON_UNKNOWN:
		return utils.DropReason_Drop_Unknown
	default:
		return utils.DropReason_Drop_Failure
	}
}

// dropReasonToUint16 narrows a Retina drop reason to the uint16 the metric
// extension expects. Values are bounded by the DropReason enum which fits in
// uint16, so the narrowing is safe.
func dropReasonToUint16(r utils.DropReason) uint16 {
	//nolint:gosec // G115: DropReason enum values are small, bounded by uint16
	return uint16(r)
}
