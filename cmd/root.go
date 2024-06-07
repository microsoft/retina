// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package cmd

import (
	"fmt"
	"os"

	"github.com/cilium/cilium/pkg/hive"
	"github.com/microsoft/retina/cmd/hubble"
	"github.com/microsoft/retina/cmd/legacy"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "retina-agent",
	Short: "Retina Agent",
	Long:  "Start Retina Agent",
	Run: func(cmd *cobra.Command, args []string) {
		// Do Stuff Here
		fmt.Println("Starting Retina Agent")
	},
}

func Execute(h *hive.Hive) {
	rootCmd.AddCommand(legacy.Cmd())
	rootCmd.AddCommand(hubble.Cmd(h))

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
