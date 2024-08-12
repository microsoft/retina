// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

// Package ciliumeventobserver contains the Retina CiliumEventObserver plugin. It uses unix socket to get events from cilium and decode them to flow objects.
package ciliumeventobserver

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
	defaultAttempts   = 10
	defaultRetryDelay = 12 * time.Second
)

func New(cfg *kcfg.Config) api.Plugin {
	return &ciliumeventobserver{
		cfg:         cfg,
		l:           log.Logger().Named(string(Name)),
		retryDelay:  defaultRetryDelay,
		maxAttempts: defaultAttempts,
		sockPath:    cfg.MonitorSockPath,
	}
}

func (c *ciliumeventobserver) Name() string {
	return string(Name)
}

func (c *ciliumeventobserver) Generate(_ context.Context) error {
	return nil
}

func (c *ciliumeventobserver) Compile(_ context.Context) error {
	return nil
}

func (c *ciliumeventobserver) Init() error {
	c.p = &parser{
		l: c.l,
	}
	err := c.p.Init()
	if err != nil {
		c.l.Error("Failed to initialize ciliumeventobserver parser", zap.Error(err))
		return err
	}
	c.l.Info("Initialized ciliumeventobserver plugin")
	return nil
}

func (c *ciliumeventobserver) Start(ctx context.Context) error {
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
		c.l.Error("Error while attempting to connect to monitor socket", zap.Error(err))
		return err
	}
	return c.monitorLoop(ctx)
}

func (c *ciliumeventobserver) Stop() error {
	c.l.Info("Stopped ciliumeventobserver plugin")
	if c.connection != nil {
		c.connection.Close()
	}
	return nil
}

func (c *ciliumeventobserver) SetupChannel(ch chan *v1.Event) error {
	c.externalChannel = ch
	return nil
}

// Create a connection to the ciliumeventobserver unix socket to monitor events
func (c *ciliumeventobserver) connect(ctx context.Context) error {
	ticker := time.NewTicker(c.retryDelay)
	curAttempt := 1
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-ticker.C:
		conn, err := net.Dial("unix", c.sockPath)
		if err != nil {
			c.l.Error("Connection attempt failed", zap.Error(err))
			if curAttempt > c.maxAttempts {
				return err
			}
		} else {
			c.l.Info("Connected to cilium monitor")
			c.connection = conn
			return nil
		}
	}
	return nil
}

// reader reports connetion error
// controller to rety connection and start reader again
// if connection manager reports error, controller reports back to plugin manager

// monitor events from uds connection
func (c *ciliumeventobserver) monitorLoop(ctx context.Context) error {
	decoder := gob.NewDecoder(c.connection)
	for {
		var pl payload.Payload
		select {
		case <-ctx.Done(): // cancelled or done
			c.l.Info("Context done, exiting monitor loop")
			return nil
		default:
			if err := pl.DecodeBinary(decoder); err != nil {
				if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
					if err = c.connect(ctx); err != nil {
						return err
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
