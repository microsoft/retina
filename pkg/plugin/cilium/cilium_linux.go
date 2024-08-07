// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

// Package cilium contains the Retina Cilium plugin. It uses unix socket to get events from cilium and decode them to flow objects.
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
	defaultAttempts    = 10
	defaultRetryDelay  = 12 * time.Second
)

func New(cfg *kcfg.Config) api.Plugin {
	return &cilium{
		cfg:         cfg,
		l:           log.Logger().Named(string(Name)),
		retryDelay:  defaultRetryDelay,
		maxAttempts: defaultAttempts,
		sockPath:    MonitorSockPath1_2,
	}
}

func (c *cilium) Name() string {
	return string(Name)
}

func (c *cilium) Generate(_ context.Context) error {
	return nil
}

func (c *cilium) Compile(_ context.Context) error {
	return nil
}

func (c *cilium) Init() error {
	c.p = &parser{
		l: c.l,
	}
	err := c.p.Init()
	if err != nil {
		c.l.Error("Failed to initialize cilium parser", zap.Error(err))
		return err
	}
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

	// Connect and monitor loop
	err := c.connect(ctx)
	if err != nil {
		c.l.Error("Error while connecting and decoding cilium events", zap.Error(err))
		return err
	}
	go func() {
		c.monitorLoop(ctx)
	}()

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
func (c *cilium) connect(ctx context.Context) error {
	// Start the cilium monitor
	for attempt := 0; attempt < c.maxAttempts; attempt++ {
		select {
		case <-ctx.Done(): // Cancelled or done
			//nolint:wrapcheck // dont wrap error since it would not provide more context
			return ctx.Err()
		default:
			conn, err := net.Dial("unix", c.sockPath)
			if err != nil {
				c.connection = nil
				c.l.Error("Failed to connect to cilium monitor", zap.Error(err))
				time.Sleep(c.retryDelay)
				continue
			}
			c.l.Info("Connected to cilium monitor")
			c.connection = conn
			return nil
		}
	}
	return nil
}

// monitor events from uds connection
func (c *cilium) monitorLoop(ctx context.Context) {
	defer c.connection.Close()
	decoder := gob.NewDecoder(c.connection)
	for {
		var pl payload.Payload
		select {
		case <-ctx.Done(): // cancelled or done
			c.l.Info("Context done, exiting monitor loop")
			return
		default:
			if err := pl.DecodeBinary(decoder); err != nil {
				if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
					c.l.Warn("Connection was closed, retrying connection", zap.Error(err))
					if err = c.connect(ctx); err != nil {
						c.l.Error("Failed to reconnect to cilium monitor", zap.Error(err))
						return
					}
					continue
				}
				c.l.Warn("Failed to decode payload from cilium", zap.Error(err))
				continue
			}
			ev, err := c.p.Decode(&pl)
			if err == nil {
				c.externalChannel <- ev
			} else {
				c.l.Warn("Failed to decode cilium payload to flow", zap.Error(err))
			}
		}
	}
}
