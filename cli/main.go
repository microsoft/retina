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
	if err := cmd.Retina.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
