// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package main

import (
	"fmt"
	"os"

	"github.com/microsoft/retina/controller/legacy"
	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "retina-agent",
		Short: "Start Retina Agent",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Starting Retina Agent")
		},
	}

	rootCmd.AddCommand(legacy.Cmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
