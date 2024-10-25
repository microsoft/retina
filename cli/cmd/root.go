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

// RetinaClient for customer consume
var RetinaClient *client.Retina

// ClientConfigPath used
var ClientConfigPath = ".retinactl.json"

type Config struct {
	RetinaEndpoint string `json:"retina_endpoint"`
}

var Retina = &cobra.Command{
	Use: "kubectl-retina",
	Short: "Retina is the eBPF distributed networking observability tool for Kubernetes",
	PersistentPreRun: func(*cobra.Command, []string) {
		var config Config
		file, _ := os.ReadFile(ClientConfigPath)
		_ = json.Unmarshal([]byte(file), &config)
		RetinaClient = client.NewRetinaClient(config.RetinaEndpoint)
	},
}

func init() {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	Logger = log.Logger().Named("retina-cli")
}
