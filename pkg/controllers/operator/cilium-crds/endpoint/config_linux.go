// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package endpointcontroller

import "github.com/spf13/pflag"

const (
	// WindowsOnlyEndpoints restricts CiliumEndpoint/CiliumIdentity management to Pods on Windows nodes.
	WindowsOnlyEndpoints = "windows-only-endpoints"
	// WindowsCIDRLabels adds a cidr:<podIP>/32 label, plus the cidrgroup: labels of every
	// CiliumCIDRGroup containing the Pod IP, to the identity of Pods on Windows nodes.
	WindowsCIDRLabels = "windows-cidr-labels"
	// ReuseOperatorIdentities makes CiliumEndpoints reference the CiliumIdentities created by
	// cilium-operator (identity-management-mode=operator) instead of allocating our own.
	ReuseOperatorIdentities = "reuse-operator-identities"
)

type Config struct {
	// WindowsOnlyEndpoints leaves Pods on non-Windows nodes to the Cilium agent.
	WindowsOnlyEndpoints bool
	// WindowsCIDRLabels keeps Windows Pods selectable by CIDR policy once they have a Cilium identity.
	WindowsCIDRLabels bool
	// ReuseOperatorIdentities filters identity labels like cilium-operator does and only looks up
	// the CiliumIdentity it created, so the operator's identity is used and not garbage collected.
	ReuseOperatorIdentities bool
}

var defaultConfig = Config{}

func (c Config) Flags(flags *pflag.FlagSet) {
	flags.Bool(WindowsOnlyEndpoints, c.WindowsOnlyEndpoints,
		"Only manage CiliumEndpoints and CiliumIdentities for Pods on Windows nodes")
	flags.Bool(WindowsCIDRLabels, c.WindowsCIDRLabels,
		"Add cidr:<podIP>/32 and matching CiliumCIDRGroup labels to identities of Pods on Windows nodes so CIDR and CIDR group policy still select them")
	flags.Bool(ReuseOperatorIdentities, c.ReuseOperatorIdentities,
		"Reuse the CiliumIdentities created by cilium-operator instead of creating them. "+
			"Requires identity-management-mode=operator in kube-system/cilium-config, whose cluster-name and labels are used to compute identity labels. "+
			"Incompatible with --"+WindowsCIDRLabels)
}
