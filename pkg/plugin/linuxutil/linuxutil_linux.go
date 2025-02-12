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
	lru "github.com/hashicorp/golang-lru/v2"
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/plugin/registry"
	"github.com/pkg/errors"
	"github.com/safchain/ethtool"
	"go.uber.org/zap"
)

const defaultLimit = 2000

func init() {
	registry.Add(name, New)
}

// New creates a linuxutil plugin.
func New(cfg *kcfg.Config) registry.Plugin {
	return &linuxUtil{
		cfg: cfg,
		l:   log.Logger().Named(name),
	}
}

func (lu *linuxUtil) Name() string {
	return name
}

func (lu *linuxUtil) Generate(context.Context) error {
	return nil
}

func (lu *linuxUtil) Compile(context.Context) error {
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

func (lu *linuxUtil) SetupChannel(chan *hubblev1.Event) error {
	lu.l.Debug("Plugin does not support SetupChannel", zap.String("plugin", name))
	return nil
}

func (lu *linuxUtil) run(ctx context.Context) error {
	lu.l.Info("Running linuxutil plugin...")

	// create a LRU cache to skip unsupported interfaces
	unsupportedInterfacesCache, err := lru.New[string, struct{}](int(defaultLimit))
	if err != nil {
		lu.l.Error("failed to create global LRU cache", zap.Error(err))
		return err
	}

	ticker := time.NewTicker(lu.cfg.MetricsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			lu.l.Info("Context is done, linuxutil will stop running")
			return nil
		case <-ticker.C:
			opts := &NetstatOpts{
				CuratedKeys:      true,
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
				errOrDropKeysOnly: true,
				addZeroVal:        false,
			}

			ethHandle, err := ethtool.NewEthtool()
			if err != nil {
				lu.l.Error("Error while creating ethHandle: %v\n", zap.Error(err))
				return fmt.Errorf("failed to create ethHandle: %w", err)
			}

			ethReader := NewEthtoolReader(ethtoolOpts, ethHandle, unsupportedInterfacesCache)
			if ethReader == nil {
				lu.l.Error("Error while creating ethReader")
				return errors.New("error while creating ethReader")
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
			ethHandle.Close()
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
