// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var config = &cobra.Command{
	Use:   "config",
	Short: "Configure retina CLI",
}

var setConfig = &cobra.Command{
	Use:   "set",
	Short: "Configure Retina client",
	RunE: func(cmd *cobra.Command, _ []string) error {
		endpoint, _ := cmd.Flags().GetString("endpoint")
		config := Config{RetinaEndpoint: endpoint}
		b, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			return err
		}
		return errors.Wrap(os.WriteFile(ClientConfigPath, b, 0o644), "failed to write config file") // nolint:gosec,gomnd // no sensitive data
	},
}

var viewConfig = &cobra.Command{
	Use:   "view",
	Short: "View Retina client config",
	RunE: func(*cobra.Command, []string) error {
		b, err := os.ReadFile(ClientConfigPath)
		if err != nil {
			return errors.Wrap(err, "failed to read config path")
		}
		fmt.Println(string(b))
		return nil
	},
}

func init() {
	setConfig.Flags().String("endpoint", "", "Set Retina server")
	_ = setConfig.MarkFlagRequired("endpoint")
	config.AddCommand(setConfig)
	config.AddCommand(viewConfig)
	Retina.AddCommand(config)
}
