// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package standard

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

var onlyOneSignalHandler = make(chan struct{})

func setupSignalHandler() context.Context {
	close(onlyOneSignalHandler)

	ctx, cancel := context.WithCancel(context.Background())
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	setupShutdownMarker(ctx, cancel)

	go func() {
		<-signals
		cancel()
		<-signals
		os.Exit(1)
	}()

	return ctx
}
