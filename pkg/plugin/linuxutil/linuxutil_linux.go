// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

// Package linuxutil contains the Retina linuxutil plugin. It gathers TCP/UDP statistics and network interface statistics from the netstats and ethtool node utilities (respectively).
package linuxutil

import (
	"context"
	"fmt"
	"sync"
	"time"

	hubblev1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/plugin/api"
	"github.com/safchain/ethtool"
	"go.uber.org/zap"
)

// New creates a linuxutil plugin.
func New(cfg *kcfg.Config) api.Plugin {
	return &linuxUtil{
		cfg: cfg,
		l:   log.Logger().Named(string(Name)),
	}
}

func (lu *linuxUtil) Name() string {
	return string(Name)
}

func (lu *linuxUtil) Generate(ctx context.Context) error {
	return nil
}

func (lu *linuxUtil) Compile(ctx context.Context) error {
	return nil
}

func (lu *linuxUtil) Init() error {
	lu.l.Info("Initializing linuxutil plugin...")
	return nil
}

func (lu *linuxUtil) Start(ctx context.Context) error {
	lu.isRunning = true
	return lu.run(ctx)
}

func (lu *linuxUtil) SetupChannel(ch chan *hubblev1.Event) error {
	lu.l.Debug("Plugin does not support SetupChannel", zap.String("plugin", string(Name)))
	return nil
}

func (lu *linuxUtil) run(ctx context.Context) error {
	lu.l.Info("Running linuxutil plugin...")
	ticker := time.NewTicker(lu.cfg.MetricsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			lu.l.Info("Context is done, linuxutil will stop running")
			return nil
		case <-ticker.C:
			opts := &NetstatOpts{
				CuratedKeys:      false,
				AddZeroVal:       false,
				ListenSock:       false,
				PrevTCPSockStats: lu.prevTCPSockStats,
			}
			var wg sync.WaitGroup

			ns := &Netstat{}
			nsReader := NewNetstatReader(opts, ns)
			wg.Add(1)
			go func() {
				defer wg.Done()
				tcpSocketStats, err := nsReader.readAndUpdate()
				if err != nil {
					lu.l.Error("Reading netstat failed", zap.Error(err))
				}
				lu.prevTCPSockStats = tcpSocketStats
			}()

			ethtoolOpts := &EthtoolOpts{
				errOrDropKeysOnly: false,
				addZeroVal:        false,
			}

			ethHandle, err := ethtool.NewEthtool()
			if err != nil {
				lu.l.Error("Error while creating ethHandle: %v\n", zap.Error(err))
				return err
			}

			ethReader := NewEthtoolReader(ethtoolOpts, ethHandle)
			if ethReader == nil {
				lu.l.Error("Error while creating ethReader")
				return fmt.Errorf("error while creating ethReader")
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := ethReader.readAndUpdate()
				if err != nil {
					lu.l.Error("Reading ethTool failed", zap.Error(err))
				}
			}()

			wg.Wait()
		}
	}
}

func (lu *linuxUtil) Stop() error {
	if !lu.isRunning {
		return nil
	}
	lu.l.Info("Stopping linuxutil plugin...")
	lu.isRunning = false
	return nil
}
