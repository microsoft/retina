// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// ConfigCmd :- Configure retina CLI
func ConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Configure retina CLI",
	}
	cmd.AddCommand(configView())
	cmd.AddCommand(configSet())
	return cmd
}

func configSet() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Configure Retina client",
		RunE: func(cmd *cobra.Command, args []string) error {
			endpoint, _ := cmd.Flags().GetString("endpoint")
			config := Config{RetinaEndpoint: endpoint}
			b, _ := json.MarshalIndent(config, "", "  ")
			err := os.WriteFile(ClientConfigPath, b, 0o644)
			if err == nil {
				fmt.Print(string(b))
			}

			return err
		},
	}
	cmd.Flags().String("endpoint", "", "Set Retina server")
	cmd.MarkFlagRequired("endpoint") //nolint:errcheck

	return cmd
}

func configView() *cobra.Command {
	return &cobra.Command{
		Use:   "view",
		Short: "View Retina client config",
		Run: func(cmd *cobra.Command, args []string) {
			// print client config
		},
	}
}
