// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

// Package infiniband contains the Retina infiniband plugin. It gathers infiniband statistics and debug status parameters.
package infiniband

import (
	"context"
	"errors"
	"time"

	hubblev1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/plugin/api"
	"go.uber.org/zap"
)

var ErrAlreadyRunning = errors.New("infiniband plugin is already running")

// New creates a infiniband plugin.
func New(cfg *kcfg.Config) api.Plugin {
	return &infiniband{
		cfg: cfg,
		l:   log.Logger().Named(string(Name)),
	}
}

func (ib *infiniband) Name() string {
	return string(Name)
}

func (ib *infiniband) Generate(ctx context.Context) error { //nolint //implementing iface
	return nil
}

func (ib *infiniband) Compile(ctx context.Context) error { //nolint // implementing iface
	return nil
}

func (ib *infiniband) Init() error {
	return nil
}

func (ib *infiniband) Start(ctx context.Context) error {
	ib.l.Info("Starting infiniband plugin")
	ib.startLock.Lock()
	if ib.isRunning {
		return ErrAlreadyRunning
	}
	ib.isRunning = true
	ib.startLock.Unlock()
	return ib.run(ctx)
}

func (ib *infiniband) SetupChannel(ch chan *hubblev1.Event) error { // nolint // impl. iface
	ib.l.Warn("Plugin does not support SetupChannel", zap.String("plugin", string(Name)))
	return nil
}

func (ib *infiniband) run(ctx context.Context) error {
	ib.l.Info("Running infiniband plugin...")
	ticker := time.NewTicker(ib.cfg.MetricsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			ib.l.Info("Context is done, infiniband will stop running")
			return nil
		case <-ticker.C:
			infinibandReader := NewInfinibandReader()
			err := infinibandReader.readAndUpdate()
			if err != nil {
				ib.l.Error("Reading infiniband stats failed", zap.Error(err))
			}
		}
	}
}

func (ib *infiniband) Stop() error {
	if !ib.isRunning {
		return nil
	}
	ib.l.Info("Stopping infiniband plugin...")
	ib.isRunning = false
	return nil
}
