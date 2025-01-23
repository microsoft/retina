package pktmon

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/pkg/errors"

	observerv1 "github.com/cilium/cilium/api/v1/observer"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/utils"
	"go.uber.org/zap"
	"go.uber.org/zap/zapio"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	ErrNilStream = errors.New("stream is nil")
)

type GRPCManager interface {
	SetupStream() error
	StartStream(ctx context.Context) error
	ReceiveFromStream() (*observerv1.GetFlowsResponse, error)

	RunPktMonServer(ctx context.Context) error
	Stop() error
}

type PktmonGRPCManager struct {
	grpcClient *GRPCClient
	stream     observerv1.Observer_GetFlowsClient
	l          *log.ZapLogger

	pktmonCmd *exec.Cmd
	stdWriter *zapio.Writer
	errWriter *zapio.Writer
}

func (p *PktmonGRPCManager) SetupStream() error {
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

func (p *PktmonGRPCManager) StartStream(ctx context.Context) error {
	if p.grpcClient == nil {
		return errors.Wrapf(ErrNilGrpcClient, "unable to start stream")
	}

	var err error
	fn := func() error {
		p.stream, err = p.grpcClient.GetFlows(ctx, &observerv1.GetFlowsRequest{})
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

func (p *PktmonGRPCManager) ReceiveFromStream() (*observerv1.GetFlowsResponse, error) {
	if p.stream == nil {
		return nil, errors.Wrapf(ErrNilStream, "unable to receive from stream")
	}

	return p.stream.Recv()
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

func (p *PktmonGRPCManager) RunPktMonServer(ctx context.Context) error {
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

func (p *PktmonGRPCManager) Stop() error {
	if p.pktmonCmd != nil {
		err := p.pktmonCmd.Process.Kill()
		if err != nil {
			return errors.Wrapf(err, "failed to kill pktmon server during stop")
		}
	}
	return nil
}
