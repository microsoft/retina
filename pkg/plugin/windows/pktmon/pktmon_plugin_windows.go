package pktmon

import (
	"context"
	"errors"
	"fmt"
	golog "log"

	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/metrics"
	"github.com/microsoft/retina/pkg/plugin/api"
	"github.com/microsoft/retina/pkg/utils"
)

var (
	ErrNilEnricher error = errors.New("enricher is nil")
)

const (
	Name = "pktmon"
)

type PktMonPlugin struct {
	enricher        enricher.EnricherInterface
	externalChannel chan *v1.Event
	pkt             PktMon
	l               *log.ZapLogger
}

func (p *PktMonPlugin) Compile(ctx context.Context) error {
	return nil
}

func (p *PktMonPlugin) Generate(ctx context.Context) error {
	return nil
}

func (p *PktMonPlugin) Init() error {
	p.pkt = &WinPktMon{
		l: log.Logger().Named(Name),
	}
	p.l = log.Logger().Named(Name)

	return nil
}

func (p *PktMonPlugin) Name() string {
	return "pktmon"
}

func (p *PktMonPlugin) SetupChannel(ch chan *v1.Event) error {
	p.externalChannel = ch
	return nil
}

func New(cfg *kcfg.Config) api.Plugin {
	return &PktMonPlugin{}
}

type DNSRequest struct {
	SourceIP      byte
	DestinationIP byte
}

func (p *PktMonPlugin) Start(ctx context.Context) error {
	fmt.Printf("setting up enricher since pod level is enabled \n")
	p.enricher = enricher.Instance()
	if p.enricher == nil {
		return ErrNilEnricher
	}

	// calling packet capture routine concurrently
	golog.Println("Starting (go)")
	err := p.pkt.Initialize()
	if err != nil {
		return fmt.Errorf("Failed to initialize pktmon: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("pktmon context cancelled: %v", ctx.Err())
		default:
			fl, meta, packet, err := p.pkt.GetNextPacket()
			if errors.Is(err, ErrNotSupported) {
				continue
			}

			if err != nil {
				golog.Printf("Error getting packet: %v\n", err)
				continue
			}

			// do this here instead of GetNextPacket to keep higher level
			// packet parsing out of L4 parsing
			err = parseDNS(fl, meta, packet)
			if err != nil {
				golog.Printf("Error parsing DNS: %v\n", err)
				continue
			}

			ev := &v1.Event{
				Event:     fl,
				Timestamp: fl.Time,
			}

			if p.enricher != nil {
				p.enricher.Write(ev)
			} else {
				fmt.Printf("enricher is nil when writing\n")
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

func (p *PktMonPlugin) Stop() error {
	return nil
}
