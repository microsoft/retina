package main

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

import (
	"fmt"
	"os"

	"github.com/cilium/cilium/hubble/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
