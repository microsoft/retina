// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package cmd

import (
	"encoding/json"
	"os"

	"github.com/microsoft/retina/pkg/client"
	"github.com/microsoft/retina/pkg/log"
	"github.com/spf13/cobra"
)

var Logger *log.ZapLogger

func init() {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	Logger = log.Logger().Named("retina-cli")
}

// RetinaClient for customer consume
var RetinaClient *client.Retina

// ClientConfigPath used
var ClientConfigPath = ".retinactl.json"

type Config struct {
	RetinaEndpoint string `json:"retina_endpoint"`
}

// NewRootCmd returns a root
func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			var config Config
			file, _ := os.ReadFile(ClientConfigPath)
			_ = json.Unmarshal([]byte(file), &config)
			RetinaClient = client.NewRetinaClient(config.RetinaEndpoint)
		},
	}
	return rootCmd
}
