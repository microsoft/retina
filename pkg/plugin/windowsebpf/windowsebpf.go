package windowsebpf

import (
	"context"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	hp "github.com/cilium/cilium/pkg/hubble/parser"

	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/metrics"
	"github.com/microsoft/retina/pkg/plugin/registry"
	"github.com/microsoft/retina/pkg/utils"
	"go.uber.org/zap"
	"go.uber.org/zap/zapio"
	"golang.org/x/sync/errgroup"
)

const (
	name = "windowsebpf"
)

var (
	ErrNilEnricher = errors.New("nil enricher")
)

type Plugin struct {
	enricher        enricher.EnricherInterface
	externalChannel chan *v1.Event
	l               *log.ZapLogger
	stdWriter       *zapio.Writer
	errWriter       *zapio.Writer

	parser *hp.Parser
}

func init() {
	registry.Add(name, New)
}

func New(*kcfg.Config) registry.Plugin {
	return &Plugin{
		l: log.Logger().Named(name),
	}
}

func (p *Plugin) Init() error {
	return nil
}

func (p *Plugin) Name() string {
	return "windowsebpf"
}

func (p *Plugin) Start(ctx context.Context) error {
	p.enricher = enricher.Instance()
	if p.enricher == nil {
		return ErrNilEnricher
	}

	g, ctx := errgroup.WithContext(ctx)

	parser, err := hp.New(logrus.WithField("cilium", "parser"),
		// We use noOp getters here since we will use our own custom parser in hubble
		nil, // todo: implement NoopEndpointGetter
		nil, // todo: implement NoopIdentityGetter
		nil, // todo: implement NoopDNSGetter
		nil, // todo: implement NoopIPGetter
		nil, // todo: implement NoopServiceGetter
		nil, // todo: implement NoopLinkGetter
		nil, // todo: implement NoopPodMetadataGetter
	)
	if err != nil {
		p.l.Fatal("Failed to create parser", zap.Error(err))
		return err //nolint:wrapcheck // dont wrap error since it would not provide more context
	}
	p.parser = parser

	for {
		select {
		case <-ctx.Done():
			return errors.Wrapf(ctx.Err(), "windowsebpf plugin context done")
		default:
			event, err := p.windowsebpf.Recv() //todo: implement windowsebpf.Recv() or
			if err != nil {
				return errors.Wrapf(err, "failed to receive windowsebpf event")
			}

			fl := event.GetFlow()
			if fl == nil {
				p.l.Error("received nil flow, flow proto mismatch from client/server?")
				return nil
			}

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
					metrics.LostEventsCounter.WithLabelValues(utils.ExternalChannel, name).Inc()
				}
			}
		}
	}
}

func (p *Plugin) SetupChannel(ch chan *v1.Event) error {
	p.externalChannel = ch
	return nil
}

func (p *Plugin) Stop() error {
	return nil
}

func (p *Plugin) Compile(context.Context) error {
	return nil
}

func (p *Plugin) Generate(context.Context) error {
	return nil
}
