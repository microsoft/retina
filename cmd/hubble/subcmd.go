// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package hubble

import (
	"fmt"
	"os"

	"github.com/cilium/cilium/pkg/hive"
	"github.com/cilium/cilium/pkg/option"
	"github.com/spf13/cobra"
	"go.etcd.io/etcd/version"
)

func Cmd(agentHive *hive.Hive) *cobra.Command {
	// agentHive := hive.New(Agent)
	hubbleCmd := &cobra.Command{
		Use:   "hubble-control-plane",
		Short: "Start Hubble control plane",
		Run: func(cobraCmd *cobra.Command, _ []string) {
			if v, _ := cobraCmd.Flags().GetBool("version"); v {
				fmt.Printf("%s %s\n", cobraCmd.Name(), version.Version)
				os.Exit(0)
			}
			if err := agentHive.Run(); err != nil {
				logger.Fatal(err)
			}
		},
	}

	agentHive.RegisterFlags(hubbleCmd.Flags())
	hubbleCmd.AddCommand(
		// cmdrefCmd,
		agentHive.Command(),
	)

	InitGlobalFlags(hubbleCmd, agentHive.Viper())

	cobra.OnInitialize(
		option.InitConfig(hubbleCmd, "retina-agent", "retina", agentHive.Viper()),

		// Populate the config and initialize the logger early as these
		// are shared by all commands.
		func() {
			initDaemonConfig(agentHive.Viper())
		},
		initLogging,
	)

	return hubbleCmd
}
