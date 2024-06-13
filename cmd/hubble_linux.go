// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package cmd

import (
	"fmt"

	"github.com/cilium/cilium/pkg/hive"
	"github.com/microsoft/retina/cmd/hubble"
	"github.com/spf13/cobra"
	"go.etcd.io/etcd/version"
)

var (
	h = hive.New(hubble.Agent)

	hubbleCmd = &cobra.Command{
		Use:   "hubble-control-plane",
		Short: "Start Hubble control plane",
		Run: func(cobraCmd *cobra.Command, _ []string) {
			if v, _ := cobraCmd.Flags().GetBool("version"); v {
				fmt.Printf("%s %s\n", cobraCmd.Name(), version.Version)
			}
			hubble.Execute(cobraCmd, h)
		},
	}
)

func init() {
	h.RegisterFlags(hubbleCmd.Flags())
	hubbleCmd.AddCommand(h.Command())

	hubble.InitGlobalFlags(hubbleCmd, h.Viper())

	rootCmd.AddCommand(hubbleCmd)
}
