// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package main

import (
	"fmt"
	"os"

	"github.com/microsoft/retina/cli/cmd"
	_ "github.com/microsoft/retina/cli/cmd/capture"
)

func main() {
	if len(os.Args) == 1 {
		// No arguments: launch TUI
		args, err := RunTUI()
		if err != nil {
			fmt.Println("TUI error:", err)
			os.Exit(1)
		}
		if args != nil {
			fmt.Printf("\nRetina CLI args: %s\n", args)
			fmt.Print("\nWould you like to run this command? (y/n): ")
			var confirm string
			_, scanErr := fmt.Scanln(&confirm)
			if scanErr != nil {
				fmt.Println("Error reading input:", scanErr)
				os.Exit(1)
			}
			if confirm == "y" || confirm == "Y" {
				cmd.Retina.SetArgs(args)
				if err := cmd.Retina.Execute(); err != nil {
					fmt.Println(err)
					os.Exit(1)
				}
			} else {
				fmt.Println("Command not executed.")
			}
		}
		// If args is nil, user cancelled or did not confirm; just exit
		return
	}
	if err := cmd.Retina.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
