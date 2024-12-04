// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

// Package dns contains the Retina DNS plugin. It uses the Inspektor Gadget DNS tracer to capture DNS events.
package dns

import (
	"context"
	"net"
	"os"

	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	"github.com/inspektor-gadget/inspektor-gadget/pkg/gadgets/trace/dns/tracer"
	"github.com/inspektor-gadget/inspektor-gadget/pkg/gadgets/trace/dns/types"
	"github.com/inspektor-gadget/inspektor-gadget/pkg/utils/host"
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/metrics"
	"github.com/microsoft/retina/pkg/plugin/common"
	"github.com/microsoft/retina/pkg/plugin/registry"
	"github.com/microsoft/retina/pkg/utils"
	"go.uber.org/zap"
)

func init() {
	registry.Add(name, New)
}

func New(cfg *kcfg.Config) registry.Plugin {
	return &dns{
		cfg: cfg,
		l:   log.Logger().Named(name),
	}
}

func (d *dns) Name() string {
	return name
}

func (d *dns) Generate(ctx context.Context) error {
	return nil
}

func (d *dns) Compile(ctx context.Context) error {
	return nil
}

func (d *dns) Init() error {
	// Create tracer. In this case no parameters are passed.
	err := host.Init(host.Config{})
	tracer, err := tracer.NewTracer()
	if err != nil {
		d.l.Error("Failed to create tracer", zap.Error(err))
		return err
	}
	d.tracer = tracer
	d.tracer.SetEventHandler(d.eventHandler)
	d.pid = uint32(os.Getpid())
	d.l.Info("Initialized dns plugin")
	return nil
}

func (d *dns) Start(ctx context.Context) error {
	if d.cfg.EnablePodLevel {
		if enricher.IsInitialized() {
			d.enricher = enricher.Instance()
		} else {
			d.l.Warn("retina enricher is not initialized")
		}
	}
	if err := d.tracer.Attach(d.pid); err != nil {
		d.l.Error("Failed to attach tracer", zap.Error(err))
		return err
	}

	<-ctx.Done()
	return nil
}

func (d *dns) Stop() error {
	if d.tracer != nil {
		d.tracer.Detach(d.pid)
		d.tracer.Close()
	}
	d.l.Info("Stopped dns plugin")
	return nil
}

func (d *dns) SetupChannel(c chan *v1.Event) error {
	d.externalChannel = c
	return nil
}

func (d *dns) eventHandler(event *types.Event) {
	if event == nil {
		return
	}
	d.l.Debug("Event received", zap.Any("event", event))

	// Update basic metrics
	if event.Qr == types.DNSPktTypeQuery {
		m = metrics.DNSRequestCounter
	} else if event.Qr == types.DNSPktTypeResponse {
		m = metrics.DNSResponseCounter
	} else {
		return
	}
	m.WithLabelValues().Inc()

	if !d.cfg.EnablePodLevel {
		return
	}

	var dir uint8
	if event.PktType == "HOST" {
		// Ingress.
		dir = 2
	} else if event.PktType == "OUTGOING" {
		// Egress.
		dir = 3
	} else {
		return
	}

	// Update advanced metrics.
	fl := utils.ToFlow(
		d.l,
		int64(event.Timestamp),
		net.ParseIP(event.SrcIP),
		net.ParseIP(event.DstIP),
		uint32(event.SrcPort),
		uint32(event.DstPort),
		uint8(common.ProtocolToFlow(event.Protocol)),
		dir,
		utils.Verdict_DNS,
	)

	meta := &utils.RetinaMetadata{}

	utils.AddDNSInfo(fl, meta, string(event.Qr), common.RCodeToFlow(event.Rcode), event.DNSName, []string{event.QType}, event.NumAnswers, event.Addresses)

	// Add metadata to the flow.
	utils.AddRetinaMetadata(fl, meta)

	ev := (&v1.Event{
		Event:     fl,
		Timestamp: fl.GetTime(),
	})
	if d.enricher != nil {
		d.enricher.Write(ev)
	}

	// Send event to external channel.
	if d.externalChannel != nil {
		select {
		case d.externalChannel <- ev:
		default:
			metrics.LostEventsCounter.WithLabelValues(utils.ExternalChannel, name).Inc()
		}
	}
}
