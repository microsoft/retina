// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package dns

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"

	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/metrics"
	"github.com/microsoft/retina/pkg/plugin/api"
	"github.com/microsoft/retina/pkg/plugin/common"
	"github.com/microsoft/retina/pkg/utils"
	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	"github.com/cilium/ebpf/rlimit"
	"github.com/inspektor-gadget/inspektor-gadget/pkg/gadgets/trace/dns/tracer"
	"github.com/inspektor-gadget/inspektor-gadget/pkg/gadgets/trace/dns/types"
	"github.com/inspektor-gadget/inspektor-gadget/pkg/utils/host"
	"go.uber.org/zap"
)

func New(cfg *kcfg.Config) api.Plugin {
	return &dns{
		cfg: cfg,
		l:   log.Logger().Named(string(Name)),
	}
}

func (d *dns) Name() string {
	return string(Name)
}

func (d *dns) Generate(ctx context.Context) error {
	return nil
}

func (d *dns) Compile(ctx context.Context) error {
	return nil
}

func (d *dns) Init() error {
	if err := rlimit.RemoveMemlock(); err != nil {
		d.l.Error("RemoveMemLock failed:%w", zap.Error(err))
		return err
	}

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
		d.enricher = enricher.Instance()
		if d.enricher == nil {
			d.l.Warn("Failed to get enricher instance")
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

	if event.Qr == types.DNSPktTypeQuery {
		m = metrics.DNSRequestCounter
	} else if event.Qr == types.DNSPktTypeResponse {
		m = metrics.DNSResponseCounter
	} else {
		return
	}
	var dir uint32
	if event.PktType == "HOST" {
		// Ingress.
		dir = 2
	} else if event.PktType == "OUTGOING" {
		// Egress.
		dir = 3
	} else {
		return
	}
	responses := strings.Join(event.Addresses, ",")
	// Update basic metrics.
	m.WithLabelValues(event.Rcode, event.QType, event.DNSName, responses, fmt.Sprintf("%d", event.NumAnswers)).Inc()

	if !d.cfg.EnablePodLevel {
		return
	}

	// Update advanced metrics.
	f := utils.ToFlow(int64(event.Timestamp), net.ParseIP(event.SrcIP),
		net.ParseIP(event.DstIP), uint32(event.SrcPort), uint32(event.DstPort),
		uint8(common.ProtocolToFlow(event.Protocol)),
		dir, utils.Verdict_DNS, 0)
	utils.AddDnsInfo(f, string(event.Qr), common.RCodeToFlow(event.Rcode), event.DNSName, []string{event.QType}, event.NumAnswers, event.Addresses)
	// d.l.Debug("DNS Flow", zap.Any("flow", f))

	ev := (&v1.Event{
		Event:     f,
		Timestamp: f.Time,
	})
	if d.enricher != nil {
		d.enricher.Write(ev)
	}

	// Send event to external channel.
	if d.externalChannel != nil {
		select {
		case d.externalChannel <- ev:
		default:
			metrics.LostEventsCounter.WithLabelValues(utils.ExternalChannel, string(Name)).Inc()
		}
	}
}
