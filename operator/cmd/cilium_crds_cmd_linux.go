// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package cmd

import (
	"fmt"

	"github.com/cilium/cilium/pkg/hive"
	"github.com/cilium/cilium/pkg/option"
	ciliumcrds "github.com/microsoft/retina/operator/cmd/cilium-crds"
	"github.com/spf13/cobra"
)

var (
	h   = hive.New(ciliumcrds.Operator)
	cmd = &cobra.Command{
		Use:   "manage-cilium-crds",
		Short: "Start the Retina operator for Hubble control plane",
		Run: func(cobraCmd *cobra.Command, _ []string) {
			fmt.Println("Starting Retina Operator with Cilium CRDs")
			ciliumcrds.Execute(h)
		},
	}
)

func init() {
	h.RegisterFlags(cmd.Flags())
	cmd.AddCommand(h.Command(), ciliumcrds.MetricsCmd)

	ciliumcrds.InitGlobalFlags(cmd, h.Viper())

	// Enable fallback to direct API probing to check for support of Leases in
	// case Discovery API fails.
	h.Viper().Set(option.K8sEnableAPIDiscovery, true)

	// not sure where flags hooks is set
	for _, hook := range ciliumcrds.FlagsHooks {
		hook.RegisterProviderFlag(cmd, h.Viper())
	}

	cobra.OnInitialize(
		option.InitConfig(cmd, "Retina-Operator", "retina-operators", h.Viper()),
	)

	rootCmd.AddCommand(cmd)
}
