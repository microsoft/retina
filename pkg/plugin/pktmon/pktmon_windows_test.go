package pktmon

import (
	"context"
	"testing"

	observerv1 "github.com/cilium/cilium/api/v1/observer"
	"github.com/microsoft/retina/pkg/log"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type TestGRPCManager struct {
	streamErrorIndex int
	streamErrors     []error
}

func (p *TestGRPCManager) SetupStream() error {
	return nil
}

func (p *TestGRPCManager) RunPktMonServer(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (p *TestGRPCManager) StartStream(_ context.Context) error {
	return nil
}

func (p *TestGRPCManager) ReceiveFromStream() (*observerv1.GetFlowsResponse, error) {
	err := p.streamErrors[p.streamErrorIndex]
	p.streamErrorIndex++
	return nil, err
}

func (p *TestGRPCManager) Stop() error {
	return nil
}

func TestStart(t *testing.T) {
	opts := log.GetDefaultLogOpts()
	_, err := log.SetupZapLogger(opts)
	require.NoError(t, err)

	p := &Plugin{
		l: log.Logger().Named("test"),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p.grpcManager = &TestGRPCManager{
		streamErrors: []error{
			status.Errorf(codes.Unavailable, "frame too large"),
			status.Errorf(codes.Internal, "unexpected EOF"),
			status.Errorf(codes.Internal, "exceeding the limit"),
			status.Errorf(codes.Internal, "cannot parse invalid wire-format data"),
			status.Errorf(codes.Internal, "string field contains invalid UTF-8"),
			status.Errorf(codes.Canceled, "context canceled"),
		},
	}

	// Start the Plugin.
	err = p.Start(ctx)
	// Check if the error is nil.
	if stat, ok := status.FromError(err); ok {
		if stat.Code() != codes.Canceled {
			t.Errorf("expected %v, got %v", codes.Canceled, stat.Code())
		}
	}
}
