package pktmon

import (
	"context"

	"github.com/pkg/errors"

	"github.com/cilium/cilium/api/v1/flow"
	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	"github.com/google/gopacket"
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/metrics"
	"github.com/microsoft/retina/pkg/plugin/api"
	"github.com/microsoft/retina/pkg/plugin/windows/pktmon/stream"
	"github.com/microsoft/retina/pkg/utils"
	"go.uber.org/zap"
)

var (
	ErrNilEnricher  = errors.New("enricher is nil")
	ErrNotSupported = errors.New("not supported")
)

const (
	Name             = "pktmon"
	eventChannelSize = 1000
)

type PktMonConn interface {
	Initialize() error
	PrintAndResetMissedWrite(sessionID string)
	PrintAndResetMissedRead(sessionID string)
	ParseDNS(fl *flow.Flow, metadata *utils.RetinaMetadata, packet gopacket.Packet) error
	GetNextPacket(ctx context.Context) (*flow.Flow, *utils.RetinaMetadata, gopacket.Packet, error)
}

type Plugin struct {
	enricher        enricher.EnricherInterface
	externalChannel chan *v1.Event
	l               *log.ZapLogger
	pkt             PktMonConn
}

func (p *Plugin) Init() error {
	return nil
}

func (p *Plugin) Name() string {
	return "pktmon"
}

func (p *Plugin) Start(ctx context.Context) error {
	p.enricher = enricher.Instance()
	if p.enricher == nil {
		return ErrNilEnricher
	}

	p.pkt = stream.NewWinPktMonStreamer(p.l, 0, 0, 0)

	for {
		select {
		case <-ctx.Done():
			return errors.Wrapf(ctx.Err(), "pktmon plugin context done")
		default:
			fl, meta, packet, err := p.pkt.GetNextPacket(ctx)
			if fl == nil {
				continue
			}

			if err != nil {
				p.l.Error("error getting packet", zap.Error(err))
				continue
			}

			// do this here instead of GetNextPacket to keep higher level
			// packet parsing out of L4 parsing
			err = p.pkt.ParseDNS(fl, meta, packet)
			if err != nil {
				p.l.Error("failed to parse DNS", zap.Error(err))
				continue
			}

			utils.AddRetinaMetadata(fl, meta)

			ev := &v1.Event{
				Event:     fl,
				Timestamp: fl.GetTime(),
			}

			if p.enricher != nil {
				p.enricher.Write(ev)
			} else {
				p.l.Error("enricher is nil when writing event")
			}

			// Write the event to the external channel.
			if p.externalChannel != nil {
				select {
				case p.externalChannel <- ev:
				default:
					// Channel is full, drop the event.
					// We shouldn't slow down the reader.
					metrics.LostEventsCounter.WithLabelValues(utils.ExternalChannel, string(Name)).Inc()
				}
			}
		}
	}
}

func (p *Plugin) SetupChannel(ch chan *v1.Event) error {
	p.externalChannel = ch
	return nil
}

func New(_ *kcfg.Config) api.Plugin {
	return &Plugin{
		l: log.Logger().Named(Name),
	}
}

func (p *Plugin) Stop() error {
	return nil
}

func (p *Plugin) Compile(_ context.Context) error {
	return nil
}

func (p *Plugin) Generate(_ context.Context) error {
	return nil
}
