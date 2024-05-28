package pktmon

import (
	"context"
	"errors"
	"fmt"
	"os/exec"

	observerv1 "github.com/cilium/cilium/api/v1/observer"
	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/metrics"
	"github.com/microsoft/retina/pkg/plugin/api"
	"github.com/microsoft/retina/pkg/utils"
	"go.uber.org/zap"
	"go.uber.org/zap/zapio"
	"google.golang.org/grpc"
)

var (
	ErrNilEnricher error = errors.New("enricher is nil")
	client         *pktMonClient
	socket         = "/tmp/retina-pktmon.sock"
)

const (
	Name = "pktmon"
)

type PktMonPlugin struct {
	enricher        enricher.EnricherInterface
	externalChannel chan *v1.Event
	pkt             PktMon
	l               *log.ZapLogger
	pktmonCmd       *exec.Cmd
	stdWriter       *zapio.Writer
}

func (p *PktMonPlugin) Init() error {

	return nil
}

func (p *PktMonPlugin) Name() string {
	return "pktmon"
}

type pktMonClient struct {
	observerv1.ObserverClient
}

func NewClient() (*pktMonClient, error) {
	retryPolicy := `{
		"methodConfig": [{
			"waitForReady": true,
			"retryPolicy": {
				"MaxAttempts": 4,
				"InitialBackoff": ".01s",
				"MaxBackoff": ".01s",
				"BackoffMultiplier": 1.0,
				"RetryableStatusCodes": [ "UNAVAILABLE" ]
			}
		}]
	}`

	conn, err := grpc.Dial(fmt.Sprintf("%s:%s", "unix", socket), grpc.WithInsecure(), grpc.WithDefaultServiceConfig(retryPolicy))
	if err != nil {
		return nil, err
	}

	return &pktMonClient{observerv1.NewObserverClient(conn)}, nil
}

func (p *PktMonPlugin) Start(ctx context.Context) error {
	p.stdWriter = &zapio.Writer{Log: p.l.Logger, Level: zap.InfoLevel}
	defer p.stdWriter.Close()
	p.pktmonCmd = exec.Command("controller-pktmon.exe")
	p.pktmonCmd.Args = append(p.pktmonCmd.Args, "--socketpath", socket)
	p.pktmonCmd.Stdout = p.stdWriter
	p.pktmonCmd.Stderr = p.stdWriter

	p.l.Info("setting up enricher since pod level is enabled \n")
	p.enricher = enricher.Instance()
	if p.enricher == nil {
		return ErrNilEnricher
	}

	p.l.Info("calling start on pktmon stream server", zap.String("cmd", p.pktmonCmd.String()))
	err := p.pktmonCmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start pktmon stream server: %w", err)
	}

	p.l.Info("creating pktmon client")
	fn := func() error {
		client, err = NewClient()
		if err != nil {
			return err
		}
		return nil
	}
	err = utils.Retry(fn, 10)
	if err != nil {
		return fmt.Errorf("failed to create pktmon client: %w", err)
	}

	str, err := client.GetFlows(ctx, &observerv1.GetFlowsRequest{})
	if err != nil {
		return fmt.Errorf("failed to open pktmon stream: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("pktmon context cancelled: %v", ctx.Err())
		default:
			event, err := str.Recv()
			if err != nil {
				p.l.Error("failed to receive pktmon event", zap.Error(err))
			}

			fl := event.GetFlow()
			if fl == nil {
				p.l.Error("received nil flow")
				continue
			}

			ev := &v1.Event{
				Event:     event.GetFlow(),
				Timestamp: event.GetFlow().Time,
			}

			if p.enricher != nil {
				p.enricher.Write(ev)
			} else {
				fmt.Printf("enricher is nil when writing\n  ")
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

func (p *PktMonPlugin) SetupChannel(ch chan *v1.Event) error {
	p.externalChannel = ch
	return nil
}

func New(cfg *kcfg.Config) api.Plugin {
	return &PktMonPlugin{
		l: log.Logger().Named(Name),
	}
}

func (p *PktMonPlugin) Stop() error {
	//p.pktmonCmd.Process.Kill()
	//p.pktmonCmd.Wait()
	//p.stdWriter.Close()
	return nil
}

func (p *PktMonPlugin) Compile(ctx context.Context) error {
	return nil
}

func (p *PktMonPlugin) Generate(ctx context.Context) error {
	return nil
}
