// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package cmd

import (
	"fmt"

	"github.com/microsoft/retina/cmd/standalone"
	"github.com/microsoft/retina/internal/buildinfo"
	"github.com/spf13/cobra"
)

var standaloneCmd = &cobra.Command{
	Use:   "standalone",
	Short: "Start Retina without K8s control plane",
	RunE: func(cobraCmd *cobra.Command, _ []string) error {
		if v, _ := cobraCmd.Flags().GetBool("version"); v {
			fmt.Printf("%s %s\n", cobraCmd.Name(), buildinfo.Version)
		}
		d := standalone.NewDaemon(cfgFile)
		if err := d.Start(); err != nil {
			return fmt.Errorf("starting standalone daemon: %w", err)
		}
		return nil
	},
}

func init() {
	standaloneCmd.Flags().AddFlagSet(rootCmd.Flags())
	rootCmd.AddCommand(standaloneCmd)
}
