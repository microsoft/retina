package pktmon

import (
	"context"
	"strings"

	"github.com/pkg/errors"

	observerv1 "github.com/cilium/cilium/api/v1/observer"
	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/metrics"
	"github.com/microsoft/retina/pkg/plugin/registry"
	"github.com/microsoft/retina/pkg/utils"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	ErrNilEnricher    = errors.New("enricher is nil")
	ErrUnexpectedExit = errors.New("unexpected exit")
	ErrNilGrpcClient  = errors.New("grpc client is nil")

	socket = "/temp/retina-pktmon.sock"
)

const (
	name                    = "pktmon"
	connectionRetryAttempts = 5
	eventChannelSize        = 1000
)

type Plugin struct {
	enricher        enricher.EnricherInterface
	externalChannel chan *v1.Event
	l               *log.ZapLogger

	grpcManager GRPCManager
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
	p.grpcManager = &PktmonGRPCManager{
		l: p.l,
	}
	return nil
}

func (p *Plugin) Name() string {
	return "pktmon"
}

type GRPCClient struct {
	observerv1.ObserverClient
}

func (p *Plugin) Start(ctx context.Context) error {
	p.enricher = enricher.Instance()
	if p.enricher == nil {
		return ErrNilEnricher
	}

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		err := p.grpcManager.RunPktMonServer(ctx)
		if err != nil {
			return errors.Wrapf(err, "pktmon server exited")
		}
		return nil
	})

	err := p.grpcManager.SetupStream()
	if err != nil {
		return errors.Wrapf(err, "failed to setup initial pktmon stream")
	}

	// run the getflows loop
	g.Go(func() error {
		for {
			err := p.GetFlow(ctx)
			if stat, ok := status.FromError(err); ok {

				if stat.Code() == codes.Unavailable && strings.Contains(err.Error(), "frame too large") { // so far it's the only retriable error
					p.l.Error("failed to get flow, frame issue:", zap.Error(err))
					continue
				}

				// commonly seen with:
				// {"error":"failed to receive pktmon event: rpc error: code = Internal desc = unexpected EOF"}
				// {"error":"failed to receive pktmon event: rpc error: code = Internal desc = received 65576-bytes data exceeding the limit 65535 bytes"}
				// {"error":"failed to receive pktmon event: rpc error: code = Internal desc = grpc: failed to unmarshal the received message: proto: cannot parse invalid wire-format data"}
				// {"error":"failed to receive pktmon event: rpc error: code = Internal desc = grpc: failed to unmarshal the received message: string field contains invalid UTF-8"}
				// These errors don't impact subsequent messages received, and don't impact session connectivity,
				// so we can log them and ignore them for further investigation without restarting
				if stat.Code() == codes.Internal &&
					(strings.Contains(err.Error(), "unexpected EOF") ||
						strings.Contains(err.Error(), "exceeding the limit") ||
						strings.Contains(err.Error(), "cannot parse invalid wire-format data") ||
						strings.Contains(err.Error(), "string field contains invalid UTF-8")) {

					p.l.Error("failed to get flow, internal error:", zap.Error(err))
					continue
				}
			}
			return errors.Wrapf(err, "failed to get flow")
		}
	})

	return g.Wait()
}

func (p *Plugin) GetFlow(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	err := p.grpcManager.StartStream(ctx)
	if err != nil {
		return errors.Wrapf(err, "failed to setup pktmon stream")
	}

	for {
		select {
		case <-ctx.Done():
			return errors.Wrapf(ctx.Err(), "pktmon plugin context done")
		default:
			event, err := p.grpcManager.ReceiveFromStream()
			if err != nil {
				return errors.Wrapf(err, "failed to receive pktmon event")
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
	if p.grpcManager != nil {
		return errors.Wrapf(p.grpcManager.Stop(), "failed to stop pktmon")
	}
	return nil
}

func (p *Plugin) Compile(context.Context) error {
	return nil
}

func (p *Plugin) Generate(context.Context) error {
	return nil
}
