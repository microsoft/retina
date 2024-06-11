// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package cmd

import (
	"github.com/microsoft/retina/cmd/legacy"
	"github.com/spf13/cobra"
)

const (
	configFileName = "/retina/config/config.yaml"
)

var (
	metricsAddr          string
	probeAddr            string
	enableLeaderElection bool
	cfgFile              string

	legacyCmd = &cobra.Command{
		Use:   "legacy-control-plane",
		Short: "Start Retina legacy control plane",
		Long:  "Start Retina legacy control plane",
		Run: func(cmd *cobra.Command, args []string) {
			d := legacy.NewDaemon(metricsAddr, probeAddr, cfgFile, enableLeaderElection)
			d.Start()
		},
	}
)

func init() {
	rootCmd.AddCommand(legacyCmd)

	legacyCmd.Flags().StringVar(&metricsAddr, "metrics-bind-address", ":18080", "The address the metric endpoint binds to.")
	legacyCmd.Flags().StringVar(&probeAddr, "health-probe-bind-address", ":18081", "The address the probe endpoint binds to.")
	legacyCmd.Flags().BoolVar(&enableLeaderElection, "leader-elect", false, "Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	legacyCmd.Flags().StringVar(&cfgFile, "config", configFileName, "config file")
}
