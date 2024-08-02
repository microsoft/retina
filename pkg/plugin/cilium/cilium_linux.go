// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

// Package dns contains the Retina DNS plugin. It uses the Inspektor Gadget DNS tracer to capture DNS events.
package cilium

import (
	"context"
	"encoding/gob"
	"errors"
	"io"
	"net"
	"time"

	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	"github.com/cilium/cilium/pkg/monitor/payload"
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/plugin/api"
	"go.uber.org/zap"
)

const (
	MonitorSockPath1_2 = "/var/run/cilium/monitor1_2.sock"
	connectionTimeout  = 12
)

func New(cfg *kcfg.Config) api.Plugin {
	return &cilium{
		cfg: cfg,
		l:   log.Logger().Named(string(Name)),
	}
}

func (c *cilium) Name() string {
	return string(Name)
}

func (c *cilium) Generate(ctx context.Context) error {
	return nil
}

func (c *cilium) Compile(ctx context.Context) error {
	return nil
}

func (c *cilium) Init() error {
	c.p = &parser{
		l: c.l,
	}
	c.p.Init()
	c.l.Info("Initialized cilium plugin")
	return nil
}

func (c *cilium) Start(ctx context.Context) error {
	if c.cfg.EnablePodLevel {
		if enricher.IsInitialized() {
			c.enricher = enricher.Instance()
		} else {
			c.l.Warn("retina enricher is not initialized")
		}
	}

	// Start the cilium monitor
	go c.monitor(ctx)

	<-ctx.Done()
	return nil
}

func (c *cilium) Stop() error {

	c.l.Info("Stopped cilium plugin")
	return nil
}

func (c *cilium) SetupChannel(ch chan *v1.Event) error {
	c.externalChannel = ch
	return nil
}

// Create a connection to the cilium unix socket to monitor events
func (c *cilium) monitor(ctx context.Context) {
	// Start the cilium monitor
	for ; ; time.Sleep(12) {
		conn, err := net.Dial("unix", MonitorSockPath1_2)
		if err != nil {
			c.l.Error("Failed to connect to cilium monitor", zap.Error(err))
			continue
		}
		c.l.Info("Connected to cilium monitor")
		c.connection = conn
		err = c.monitorLoop(ctx)
		if err != nil {
			c.l.Error("Monitor loop exited with error", zap.Error(err))
		} else if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			c.l.Warn("Connection was closed")
		}
	}
}

// monitor events from uds connection
func (c *cilium) monitorLoop(ctx context.Context) error {
	defer c.connection.Close()
	decoder := gob.NewDecoder(c.connection)
	var pl payload.Payload

	for {
		select {
		case <-ctx.Done():
			c.l.Info("Context done, exiting monitor loop")
			return nil
		default:
			if err := pl.DecodeBinary(decoder); err != nil {
				c.l.Error("Failed to decode payload", zap.Error(err))
				return err
			}
			fl, err := c.p.Decode(&pl)
			if err == nil {
				c.l.Debug("Decoded flow", zap.Any("flow", fl))
				event := &v1.Event{
					Event:     fl,
					Timestamp: fl.GetTime(),
				}
				c.externalChannel <- event
			} else {
				c.l.Warn("Failed to decode to flow", zap.Error(err))
			}
		}
	}
}
