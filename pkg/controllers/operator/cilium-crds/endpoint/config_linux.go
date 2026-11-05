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
)

type Config struct {
	// WindowsOnlyEndpoints leaves Pods on non-Windows nodes to the Cilium agent.
	WindowsOnlyEndpoints bool
	// WindowsCIDRLabels keeps Windows Pods selectable by CIDR policy once they have a Cilium identity.
	WindowsCIDRLabels bool
}

var defaultConfig = Config{}

func (c Config) Flags(flags *pflag.FlagSet) {
	flags.Bool(WindowsOnlyEndpoints, c.WindowsOnlyEndpoints,
		"Only manage CiliumEndpoints and CiliumIdentities for Pods on Windows nodes")
	flags.Bool(WindowsCIDRLabels, c.WindowsCIDRLabels,
		"Add cidr:<podIP>/32 and matching CiliumCIDRGroup labels to identities of Pods on Windows nodes so CIDR and CIDR group policy still select them")
}
