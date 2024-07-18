package pktmon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/cilium/cilium/api/v1/flow"
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
	"google.golang.org/grpc/credentials/insecure"
)

var (
	ErrNilEnricher    = errors.New("enricher is nil")
	ErrUnexpectedExit = errors.New("unexpected exit")

	socket = "/temp/retina-pktmon.sock"
)

const (
	Name                    = "pktmon"
	connectionRetryAttempts = 3
)

type Plugin struct {
	enricher        enricher.EnricherInterface
	externalChannel chan *v1.Event
	l               *log.ZapLogger
	pktmonCmd       *exec.Cmd
	stdWriter       *zapio.Writer
	errWriter       *zapio.Writer
}

func (p *Plugin) Init() error {
	return nil
}

func (p *Plugin) Name() string {
	return "pktmon"
}

type Client struct {
	observerv1.ObserverClient
}

func NewClient() (*Client, error) {
	retryPolicy := map[string]any{
		"methodConfig": []map[string]any{
			{
				"waitForReady": true,
				"retryPolicy": map[string]any{
					"MaxAttempts":          4,
					"InitialBackoff":       ".01s",
					"MaxBackoff":           ".01s",
					"BackoffMultiplier":    1.0,
					"RetryableStatusCodes": []string{"UNAVAILABLE"},
				},
			},
		},
	}

	bytes, err := json.Marshal(retryPolicy)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal retry policy: %w", err)
	}

	retryPolicyStr := string(bytes)

	conn, err := grpc.Dial(fmt.Sprintf("%s:%s", "unix", socket), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDefaultServiceConfig(retryPolicyStr))
	if err != nil {
		return nil, fmt.Errorf("failed to dial pktmon server: %w", err)
	}

	return &Client{observerv1.NewObserverClient(conn)}, nil
}

func (p *Plugin) RunPktMonServer() error {
	p.stdWriter = &zapio.Writer{Log: p.l.Logger, Level: zap.InfoLevel}
	defer p.stdWriter.Close()
	p.errWriter = &zapio.Writer{Log: p.l.Logger, Level: zap.ErrorLevel}
	defer p.errWriter.Close()
	p.pktmonCmd = exec.Command("controller-pktmon.exe")
	p.pktmonCmd.Args = append(p.pktmonCmd.Args, "--socketpath", socket)
	p.pktmonCmd.Env = os.Environ()
	p.pktmonCmd.Stdout = p.stdWriter
	p.pktmonCmd.Stderr = p.errWriter

	p.l.Info("calling start on pktmon stream server", zap.String("cmd", p.pktmonCmd.String()))

	// block this thread, and should it ever return, it's a problem
	err := p.pktmonCmd.Run()
	if err != nil {
		return fmt.Errorf("pktmon server exited when it should not have: %w", err)
	}

	// we never want to return happy from this
	return fmt.Errorf("pktmon server exited unexpectedly: %w", ErrUnexpectedExit)
}

func (p *Plugin) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	p.enricher = enricher.Instance()
	if p.enricher == nil {
		return ErrNilEnricher
	}

	go func() {
		err := p.RunPktMonServer()
		if err != nil {
			p.l.Error("failed to run pktmon server", zap.Error(err))
		}

		// if the pktmon server process exits, cancel the context, we need to crash
		cancel()
	}()

	var str observerv1.Observer_GetFlowsClient
	fn := func() error {
		p.l.Info("creating pktmon client")
		client, err := NewClient()
		if err != nil {
			return fmt.Errorf("failed to create pktmon client before getting flows: %w", err)
		}

		str, err = client.GetFlows(ctx, &observerv1.GetFlowsRequest{})
		if err != nil {
			return fmt.Errorf("failed to open pktmon stream: %w", err)
		}
		return nil
	}
	err := utils.Retry(fn, connectionRetryAttempts)
	if err != nil {
		return fmt.Errorf("failed to create pktmon client: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("pktmon context cancelled: %w", ctx.Err())
		default:
			err := p.GetFlow(str)
			if err != nil {
				return fmt.Errorf("failed to get flow from observer: %w", err)
			}
		}
	}
}

func (p *Plugin) GetFlow(str observerv1.Observer_GetFlowsClient) error {
	event, err := str.Recv()
	if err != nil {
		return fmt.Errorf("failed to receive pktmon event: %w", err)
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

	if fl.GetType() == flow.FlowType_L7 {
		dns := fl.GetL7().GetDns()
		if dns != nil {
			query := dns.GetQuery()
			ans := dns.GetIps()
			if dns.GetQtypes()[0] == "Q" {
				p.l.Sugar().Debugf("query from %s to %s: request %s\n", fl.GetIP().GetSource(), fl.GetIP().GetDestination(), query)
			} else {
				p.l.Sugar().Debugf("answer from %s to %s: result: %+v\n", fl.GetIP().GetSource(), fl.GetIP().GetDestination(), ans)
			}
		}
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
	return nil
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
	if p.pktmonCmd != nil {
		err := p.pktmonCmd.Process.Kill()
		if err != nil {
			return fmt.Errorf("failed to kill pktmon server during stop: %w", err)
		}
	}

	return nil
}

func (p *Plugin) Compile(_ context.Context) error {
	return nil
}

func (p *Plugin) Generate(_ context.Context) error {
	return nil
}
