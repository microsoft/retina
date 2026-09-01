// A production EventSource for the ebpfwindows plugin that streams flows
// from the WCN / eBPF-for-Windows observability producer over a gRPC
// Observer stream on a node-local socket. This mirrors the pktmon plugin,
// which already consumes flows over the same gRPC Observer contract.
package ebpfwindows

import (
	"context"
	"sync"

	observerv1 "github.com/cilium/cilium/api/v1/observer"
	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/microsoft/retina/pkg/log"
	"go.uber.org/zap"
)

// defaultSocketPath is the node-local socket over which the WCN
// observability producer serves the gRPC Observer stream. On a Unix-HNS
// (Windows Server 2025 + Cilium-on-Windows) node the gRPC Unix transport
// uses a POSIX-style path in this form.
const defaultSocketPath = "/run/retina-ebpf-flow/retina-ebpf-flow.sock"

// ObserverSource is a production EventSource that streams flows from a WCN
// gRPC server over a node-local socket. It is safe for one Start then Stop.
type ObserverSource struct {
	mu       sync.Mutex
	sockPath string
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	conn     *grpc.ClientConn
}

// newObserverSource returns an ObserverSource reading from sockPath.
func newObserverSource(sockPath string) *ObserverSource {
	if sockPath == "" {
		sockPath = defaultSocketPath
	}
	return &ObserverSource{sockPath: sockPath}
}

// Start dials the WCN gRPC server and returns a channel of streamed flows.
func (s *ObserverSource) Start(ctx context.Context) (<-chan *v1.Event, error) {
	ctx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	s.cancel = cancel
	s.mu.Unlock()

	conn, err := grpc.Dial(
		"unix:"+s.sockPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}
	s.conn = conn

	client := observerv1.NewObserverClient(conn)
	stream, err := client.GetFlows(ctx, &observerv1.GetFlowsRequest{})
	if err != nil {
		_ = conn.Close()
		return nil, err
	}

	ch := make(chan *v1.Event, 1024)
	s.wg.Add(1)
	go s.consume(ctx, stream, ch)
	return ch, nil
}

func (s *ObserverSource) consume(ctx context.Context, stream observerv1.Observer_GetFlowsClient, ch chan<- *v1.Event) {
	defer s.wg.Done()
	defer close(ch)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		resp, err := stream.Recv()
		if err != nil {
			if ctx.Err() == nil {
				log.Logger().Named(name).Error("WCN/ebpf flow stream ended", zap.Error(err))
			}
			return
		}
		fl := resp.GetFlow()
		if fl == nil {
			continue
		}
		ev := &v1.Event{Event: fl, Timestamp: fl.GetTime()}
		select {
		case ch <- ev:
		case <-ctx.Done():
			return
		}
	}
}

func (s *ObserverSource) Stop() error {
	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
	}
	s.mu.Unlock()
	s.wg.Wait()
	if s.conn != nil {
		_ = s.conn.Close()
	}
	return nil
}
