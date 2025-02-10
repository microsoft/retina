// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package cmd

import (
	"fmt"
	"os"

	"github.com/microsoft/retina/operator/cmd/standard"
	"github.com/pkg/errors"
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
		RunE: func(_ *cobra.Command, _ []string) error {
			fmt.Println("Starting Retina Operator")
			d := standard.NewOperator(metricsAddr, probeAddr, cfgFile, enableLeaderElection)

			if err := d.Start(); err != nil {
				return errors.Wrap(err, "failed to start retina-operator")
			}
			return nil
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
