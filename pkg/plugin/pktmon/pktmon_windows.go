package pktmon

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"

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
	"go.uber.org/zap/zapio"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

var (
	ErrNilEnricher             = errors.New("enricher is nil")
	ErrUnexpectedExit          = errors.New("unexpected exit")
	ErrNilGrpcClient           = errors.New("grpc client is nil")
	ErrStreamHealthCheckFailed = errors.New("pktmon stream health check failed - likely another ETW consumer is active or stream is broken")

	socket = "/temp/retina-pktmon.sock"
)

const (
	name                       = "pktmon"
	connectionRetryAttempts    = 5
	eventChannelSize           = 1000
	eventHealthCheckFirstEvent = 60 * time.Second
	maxNilFlowsAllowed         = 5
)

type Plugin struct {
	enricher        enricher.EnricherInterface
	externalChannel chan *v1.Event
	l               *log.ZapLogger
	pktmonCmd       *exec.Cmd
	stdWriter       *zapio.Writer
	errWriter       *zapio.Writer

	grpcClient *GRPCClient
	stream     observerv1.Observer_GetFlowsClient
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
	return "pktmon"
}

type GRPCClient struct {
	observerv1.ObserverClient
}

func newGRPCClient() (*GRPCClient, error) {
	retryPolicy := map[string]any{
		"methodConfig": []map[string]any{
			{
				"waitForReady": true,
				"retryPolicy": map[string]any{
					"MaxAttempts":          connectionRetryAttempts,
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
		return nil, errors.Wrapf(err, "failed to marshal retry policy")
	}

	retryPolicyStr := string(bytes)

	conn, err := grpc.Dial(fmt.Sprintf("%s:%s", "unix", socket), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDefaultServiceConfig(retryPolicyStr))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to dial pktmon server:")
	}

	return &GRPCClient{observerv1.NewObserverClient(conn)}, nil
}

func (p *Plugin) RunPktMonServer(ctx context.Context) error {
	p.stdWriter = &zapio.Writer{Log: p.l.Logger, Level: zap.InfoLevel}
	defer p.stdWriter.Close()
	p.errWriter = &zapio.Writer{Log: p.l.Logger, Level: zap.ErrorLevel}
	defer p.errWriter.Close()

	pwd, err := os.Getwd()
	if err != nil {
		return errors.Wrapf(err, "failed to get current working directory for pktmon")
	}

	cmd := pwd + "\\" + "controller-pktmon.exe"

	p.pktmonCmd = exec.CommandContext(ctx, cmd)
	p.pktmonCmd.Dir = pwd
	p.pktmonCmd.Args = append(p.pktmonCmd.Args, "--socketpath", socket)
	p.pktmonCmd.Env = os.Environ()
	p.pktmonCmd.Stdout = p.stdWriter
	p.pktmonCmd.Stderr = p.errWriter

	p.l.Info("calling start on pktmon stream server", zap.String("cmd", p.pktmonCmd.String()))

	// block this thread, and should it ever return, it's a problem
	err = p.pktmonCmd.Run()
	if err != nil {
		return errors.Wrapf(err, "pktmon server exited when it should not have")
	}

	// we never want to return happy from this
	return errors.Wrapf(ErrUnexpectedExit, "pktmon server exited unexpectedly")
}

func (p *Plugin) Start(ctx context.Context) error {
	p.enricher = enricher.Instance()
	if p.enricher == nil {
		return ErrNilEnricher
	}

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		err := p.RunPktMonServer(ctx)
		if err != nil {
			return errors.Wrapf(err, "pktmon server exited")
		}
		return nil
	})

	err := p.SetupStream()
	if err != nil {
		return errors.Wrapf(err, "failed to setup initial pktmon stream")
	}

	// Start the stream before verifying
	err = p.StartStream(ctx)
	if err != nil {
		return errors.Wrapf(err, "failed to start initial pktmon stream")
	}

	// Verify that the event stream is producing events
	// This detects silent ETW registration failures where another consumer is already active
	err = p.verifyEventStream()
	if err != nil {
		return errors.Wrapf(err, "pktmon event stream health check failed")
	}

	// run the getflows loop
	g.Go(func() error {
		for {
			err := p.GetFlow(ctx)
			if _, ok := status.FromError(err); ok {
				p.l.Error("failed to get flow, retriable:", zap.Error(err))
				continue
			}
			return errors.Wrapf(err, "failed to get flow, unrecoverable")
		}
	})

	return g.Wait()
}

func (p *Plugin) SetupStream() error {
	var err error
	fn := func() error {
		p.l.Info("creating pktmon client")
		p.grpcClient, err = newGRPCClient()
		if err != nil {
			return errors.Wrapf(err, "failed to create pktmon client before getting flows")
		}

		return nil
	}
	err = utils.Retry(fn, connectionRetryAttempts)
	if err != nil {
		return errors.Wrapf(err, "failed to create pktmon client")
	}

	return nil
}

func (p *Plugin) StartStream(ctx context.Context) error {
	if p.grpcClient == nil {
		return errors.Wrapf(ErrNilGrpcClient, "unable to start stream")
	}

	var err error
	fn := func() error {
		// Use a long timeout for stream setup, independent of parent context cancellation
		streamCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		p.stream, err = p.grpcClient.GetFlows(streamCtx, &observerv1.GetFlowsRequest{})
		if err != nil {
			return errors.Wrapf(err, "failed to open pktmon stream")
		}
		return nil
	}
	err = utils.Retry(fn, connectionRetryAttempts)
	if err != nil {
		return errors.Wrapf(err, "failed to create pktmon client")
	}

	return nil
}

// verifyEventStream checks that the gRPC stream is responding and that events can flow.
// This detects scenarios where ETW registration silently fails because another
// consumer is already active on the EVENTS_MAP (indicated by gRPC errors).
// If the stream is healthy but no traffic is present, it logs a warning but continues.
func (p *Plugin) verifyEventStream() error {
	// Create an independent background context for the health check
	// This is NOT a child of the parent ctx to avoid cancellation when the parent shuts down
	// while the health check is still running
	healthCtx, cancel := context.WithTimeout(context.Background(), eventHealthCheckFirstEvent)
	defer cancel()

	// Create a channel to receive the result
	resultCh := make(chan error, 1)

	go func() {
		nilFlowCount := 0

		for {
			event, err := p.stream.Recv()
			if err != nil {
				resultCh <- errors.Wrapf(err, "failed to receive first event during health check")
				return
			}

			if event.GetFlow() == nil {
				nilFlowCount++
				if nilFlowCount > maxNilFlowsAllowed {
					resultCh <- errors.New("received too many nil flows during health check - likely proto mismatch")
					return
				}
				// Skip nil flows and wait for next event
				continue
			}

			// Got a valid flow
			resultCh <- nil
			return
		}
	}()

	// Wait for either a successful event or timeout
	select {
	case <-healthCtx.Done():
		// Timeout occurred - no events received, but stream connection is working.
		// This could be due to low/no traffic rather than ETW registration failure.
		p.l.Warn("no events received during health check",
			zap.Duration("timeout", eventHealthCheckFirstEvent),
			zap.String("cause", "likely no network traffic or node is idle"))
		return nil
	case err := <-resultCh:
		if err != nil {
			// Stream error indicates ETW registration failure or connection issue
			return errors.Wrapf(ErrStreamHealthCheckFailed, "%v", err)
		}
		p.l.Info("pktmon event stream verified - events flowing normally")
		return nil
	}
}

func (p *Plugin) GetFlow(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	err := p.StartStream(ctx)
	if err != nil {
		return errors.Wrapf(err, "failed to setup pktmon stream")
	}

	for {
		select {
		case <-ctx.Done():
			return errors.Wrapf(ctx.Err(), "pktmon plugin context done")
		default:
			event, err := p.stream.Recv()
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
	if p.pktmonCmd != nil {
		err := p.pktmonCmd.Process.Kill()
		if err != nil {
			return errors.Wrapf(err, "failed to kill pktmon server during stop")
		}
	}

	return nil
}

func (p *Plugin) Compile(context.Context) error {
	return nil
}

func (p *Plugin) Generate(context.Context) error {
	return nil
}
