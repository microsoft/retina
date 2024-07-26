// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

// Package dns contains the Retina DNS plugin. It uses the Inspektor Gadget DNS tracer to capture DNS events.
package cilium

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"io"
	"net"
	"time"

	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	observerTypes "github.com/cilium/cilium/pkg/hubble/observer/types"
	"github.com/cilium/cilium/pkg/monitor"
	monitorAPI "github.com/cilium/cilium/pkg/monitor/api"
	"github.com/cilium/cilium/pkg/monitor/payload"
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/plugin/api"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
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
			// monitorAgent.SendEvents - is used to notify for Agent Events (Access Log (proxy) and Agent Notify (cilium agent events - crud for ep, policy, svc))

			// hubble monitorConsumer.sendEvent -- (NotifyPerfEvent) this func sends a monitorEvent to the consumer from hubble monitor.
			// specifically, the hubble consumer adds the event to the observer's event channel

			// Agent Events
			//  - MessageTypeAccessLog:		accesslog.LogRecord
			//  - MessageTypeAgent:			api.AgentNotify
			// Perf Events
			// 	- MessageTypeDrop:			monitor.DropNotify
			// 	- MessageTypeDebug:			monitor.DebugMsg
			// 	- MessageTypeCapture:		monitor.DebugCapture
			// 	- MessageTypeTrace:			monitor.TraceNotify
			// 	- MessageTypePolicyVerdict:	monitor.PolicyVerdictNotify

			c.l.Debug("Received cilium event", zap.Int("type", pl.Type), zap.Any("payload", pl.Data))
			switch pl.Type {
			case payload.EventSample:
				data := pl.Data
				messageType := data[0]
				switch messageType {
				// Agent Events
				case monitorAPI.MessageTypeAccessLog:
					buf := bytes.NewBuffer(data[1:])
					dec := gob.NewDecoder(buf)
					lr := monitor.LogRecordNotify{}
					if err := dec.Decode(&lr); err != nil {
						c.l.Error("Failed to decode log record notify", zap.Error(err))
						continue
					}
					c.externalChannel <- &v1.Event{Timestamp: timestamppb.Now(), Event: lr}
				case monitorAPI.MessageTypeAgent:
					buf := bytes.NewBuffer(data[1:])
					dec := gob.NewDecoder(buf)
					an := monitorAPI.AgentNotifyMessage{}
					if err := dec.Decode(&an); err != nil {
						c.l.Error("Failed to decode agent notify", zap.Error(err))
						continue
					}
					c.externalChannel <- &v1.Event{Timestamp: timestamppb.Now(), Event: an}
				// Perf events
				// case monitorAPI.MessageTypeDrop:
				// 	dn := monitor.DropNotify{}
				// 	if err := binary.Read(bytes.NewReader(data), byteorder.Native, &dn); err != nil {
				// 		c.l.Error("Failed to decode drop notify", zap.Error(err))
				// 		continue
				// 	}
				// 	c.l.Info("Drop event", zap.Any("data", dn))
				// case monitorAPI.MessageTypeTrace:
				// 	tn := monitor.TraceNotify{}
				// 	if err := monitor.DecodeTraceNotify(data, &tn); err != nil {
				// 		c.l.Error("Failed to decode trace notify", zap.Error(err))
				// 		continue
				// 	}
				// 	c.l.Info("Trace event", zap.Any("data", tn))
				// case monitorAPI.MessageTypePolicyVerdict:
				// 	pn := monitor.PolicyVerdictNotify{}
				// 	if err := binary.Read(bytes.NewReader(data), byteorder.Native, &pn); err != nil {
				// 		c.l.Error("Failed to decode policy verdict notify", zap.Error(err))
				// 		continue
				// 	}
				// 	c.l.Info("Policy verdict event", zap.Any("data", pn))
				// case monitorAPI.MessageTypeDebug:
				// 	c.l.Info("Debug event", zap.Any("data", data))
				// case monitorAPI.MessageTypeCapture:
				// 	c.l.Info("Capture event", zap.Any("data", data))
				// case monitorAPI.MessageTypeRecCapture:
				// 	c.l.Info("Recorder capture event", zap.Any("data", data))
				// case monitorAPI.MessageTypeTraceSock:
				// 	c.l.Info("Trace sock event", zap.Any("data", data))
				// default:
				// 	c.l.Info("Unknown event", zap.Any("data", data))
				default:
					ev := observerTypes.PerfEvent{
						CPU:  pl.CPU,
						Data: pl.Data,
					}
					c.externalChannel <- &v1.Event{Timestamp: timestamppb.Now(), Event: ev}
				}
			case payload.RecordLost:
				c.l.Warn("Record lost for cilium event", zap.Uint64("lost", pl.Lost))
			default:
				c.l.Warn("Unknown event type", zap.Int("type", pl.Type))
				continue
			}
			// if newMA, ok := c.monitorAgent.(AgentV2); ok {
			// 	newMA.SendPerfEvent(record)
			// }
		}
	}
}
