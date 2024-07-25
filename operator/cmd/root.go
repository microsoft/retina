// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package cmd

import (
	"fmt"
	"os"

	"github.com/microsoft/retina/operator/cmd/legacy"
	"github.com/spf13/cobra"
)

const (
	configFileName = "retina/operator-config.yaml"
)

var (
	metricsAddr          string
	probeAddr            string
	enableLeaderElection bool
	cfgFile              string

	rootCmd = &cobra.Command{
		Use:   "retina-operator",
		Short: "Retina Operator",
		Long:  "Start Retina Operator",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Starting Retina Operator")
			d := legacy.NewOperator(metricsAddr, probeAddr, cfgFile, enableLeaderElection)
			d.Start()
		},
	}
)

func init() {
	rootCmd.Flags().StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	rootCmd.Flags().StringVar(&probeAddr, "probe-addr", ":8081", "The address the probe endpoint binds to.")
	rootCmd.Flags().BoolVar(&enableLeaderElection, "enable-leader-election", false, "Enable leader election for controller manager.")
	rootCmd.Flags().StringVar(&cfgFile, "config", configFileName, "config file")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
