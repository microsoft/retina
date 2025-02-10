// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package cmd

import (
	"fmt"
	"os"

	"github.com/microsoft/retina/cmd/standard"
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
	kubeConfigFileName   = ""

	rootCmd = &cobra.Command{
		Use:   "retina-agent",
		Short: "Retina Agent",
		Long:  "Start Retina Agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Do Stuff Here
			fmt.Println("Starting Retina Agent")
			d := standard.NewDaemon(metricsAddr, probeAddr, cfgFile, enableLeaderElection)
			if err := d.Start(); err != nil {
				return fmt.Errorf("starting daemon: %w", err)
			}
			return nil
		},
	}
)

func init() {
	rootCmd.Flags().StringVar(&metricsAddr, "metrics-bind-address", ":18080", "The address the metric endpoint binds to.")
	rootCmd.Flags().StringVar(&probeAddr, "health-probe-bind-address", ":18081", "The address the probe endpoint binds to.")
	rootCmd.Flags().BoolVar(&enableLeaderElection, "leader-elect", false, "Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	rootCmd.Flags().StringVar(&cfgFile, "config", configFileName, "config file")

	// this is read during GetConfigOrDie, not explicitly passed to any of our logic
	rootCmd.Flags().StringVar(&kubeConfigFileName, "kubeconfig", kubeConfigFileName, "noop we just need cobra to not check since controller runtime can use this flag")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
