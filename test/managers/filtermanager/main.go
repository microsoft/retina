// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
// nolint

package main

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/managers/filtermanager"
	"github.com/microsoft/retina/pkg/managers/watchermanager"
	"github.com/microsoft/retina/pkg/metrics"
	"github.com/microsoft/retina/pkg/watchers/apiserver"
	"github.com/microsoft/retina/pkg/watchers/endpoint"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

func main() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter comma-delimited ips for filter manager: ")
	input, _ := reader.ReadString('\n')

	opts := log.GetDefaultLogOpts()
	opts.Level = "debug"
	log.SetupZapLogger(opts)
	l := log.Logger().Named("test-packetparser")

	metrics.InitializeMetrics()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// watcher manager
	wm := watchermanager.NewWatcherManager()
	wm.Watchers = []watchermanager.Watcher{endpoint.NewWatcher(), apiserver.NewWatcher()}

	g, ctx := errgroup.WithContext(ctx)
	// Start watcher manager
	g.Go(func() error {
		err := wm.Start(ctx)
		if err != nil {
			l.Error("watcher manager exited with error", zap.Error(err))
			return errors.Wrap(err, "watcher manager exited with error")
		}
		return nil
	})
	defer func() {
		if err := wm.Stop(ctx); err != nil {
			l.Error("Failed to stop endpoint watcher", zap.Error(err))
		}
	}()

	// Filtermanager.
	f, err := filtermanager.Init(5)
	if err != nil {
		l.Error("Failed to start Filtermanager", zap.Error(err))
		panic(err)
	}
	defer func() {
		if err := f.Stop(); err != nil {
			l.Error("Failed to stop Filtermanager", zap.Error(err))
		}
	}()

	ipsStr := strings.Split(input, ",")
	var ips []net.IP
	for _, ipStr := range ipsStr {
		ip := net.ParseIP(strings.TrimSpace(ipStr))
		if ip != nil {
			ips = append(ips, ip)
		}
	}
	// Add IPs to filtermanager.
	err = f.AddIPs(ips, "packetparser-test", filtermanager.RequestMetadata{RuleID: "test"})
	if err != nil {
		l.Error("AddIPs failed", zap.Error(err))
		return
	}

	time.Sleep(30 * time.Second)
}
