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
	"github.com/microsoft/retina/pkg/metrics"
	"github.com/microsoft/retina/pkg/plugin/registry"
	"github.com/microsoft/retina/pkg/utils"
	"go.uber.org/zap"
)

const (
	defaultAttempts   = 5
	defaultRetryDelay = 12 * time.Second
	workers           = 2
	buffer            = 10000
	parserMetric      = "parser"
)

var (
	errFailedConnection = errors.New("failed to connect to cilium monitor after max attempts")
	errFailedToDial     = errors.New("failed to dial cilium monitor socket")
	errPodLevelDisabled = errors.New("pod level enricher is not initialized")
)

func init() {
	registry.Plugins[name] = New
}

func New(cfg *kcfg.Config) registry.Plugin {
	return &ciliumeventobserver{
		cfg:           cfg,
		l:             log.Logger().Named(name),
		retryDelay:    defaultRetryDelay,
		maxAttempts:   defaultAttempts,
		sockPath:      cfg.MonitorSockPath,
		payloadEvents: make(chan *payload.Payload, buffer),
		d:             &DefaultDialer{},
	}
}

func (c *ciliumeventobserver) Name() string {
	return name
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
	} else {
		return errPodLevelDisabled
	}

	for i := 0; i < workers; i++ {
		go c.parserLoop(ctx)
	}

	for {
		err := c.connect(ctx)
		if err != nil {
			c.l.Error("Error while attempting to connect to monitor socket", zap.Error(err))
			return err
		}
		// only error returned should be EOF here.
		err = c.monitorLoop(ctx)
		if err != nil {
			c.l.Error("Error while monitoring cilium events", zap.Error(err))
		}
	}
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

// Wrapper on net.Dial to mock in tests
func (d *DefaultDialer) Dial(network, address string) (net.Conn, error) {
	conn, err := net.Dial(network, address)
	if err != nil {
		return nil, errFailedToDial
	}
	return conn, nil
}

// Create a connection to the ciliumeventobserver unix socket to monitor events
func (c *ciliumeventobserver) connect(ctx context.Context) error {
	ticker := time.NewTicker(c.retryDelay)
	curAttempt := 1
	select {
	case <-ctx.Done():
		return ctx.Err() //nolint:wrapcheck // no additional context needed
	case <-ticker.C:
		conn, err := c.d.Dial("unix", c.sockPath)
		if err != nil {
			c.l.Error("Connection attempt failed", zap.Error(err))
			curAttempt++
			if curAttempt > c.maxAttempts {
				c.connection = nil
				return errFailedConnection
			}
		} else {
			c.l.Info("Connected to cilium monitor")
			c.connection = conn
			return nil
		}
	}
	return nil
}

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
					return err //nolint:wrapcheck // Error is handled by the caller
				}
				c.l.Warn("Failed to decode payload from cilium", zap.Error(err))
				metrics.LostEventsCounter.WithLabelValues(parserMetric, name).Inc()
				continue
			}
			select {
			case c.payloadEvents <- &pl:
			default:
				metrics.LostEventsCounter.WithLabelValues(utils.BufferedChannel, name).Inc()
			}
		}
	}
}

func (c *ciliumeventobserver) parserLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			c.l.Info("Context done, exiting parser loop")
			return
		case pl := <-c.payloadEvents:
			ev, err := c.p.Decode(pl)
			if err != nil {
				c.l.Warn("Failed to decode cilium payload to flow", zap.Error(err))
				continue
			}
			select {
			case c.externalChannel <- ev:
			default:
				metrics.LostEventsCounter.WithLabelValues(utils.BufferedChannel, name).Inc()
			}
		}
	}
}
