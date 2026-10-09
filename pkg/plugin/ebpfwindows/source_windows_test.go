// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

// Tests the ObserverSource (WCN gRPC flow consumer) using an in-process gRPC
// Observer server over a local Unix socket, so it runs without the WCN runtime.
package ebpfwindows

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"

	flowpb "github.com/cilium/cilium/api/v1/flow"
	observerv1 "github.com/cilium/cilium/api/v1/observer"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/microsoft/retina/pkg/log"
)

// fakeObserver implements the ObseserverServer interface, streaming the given
// flows then waiting for the client to disconnect.
type fakeObserver struct {
	observerv1.UnimplementedObserverServer
	flows []*flowpb.Flow
}

func (f *fakeObserver) GetFlows(_ *observerv1.GetFlowsRequest, srv observerv1.Observer_GetFlowsServer) error {
	for _, fl := range f.flows {
		if err := srv.Send(&observerv1.GetFlowsResponse{ResponseTypes: &observerv1.GetFlowsResponse_Flow{Flow: fl}}); err != nil {
			return fmt.Errorf("sending flow: %w", err)
		}
	}
	// Keep the stream open until the client disconnects.
	<-srv.Context().Done()
	return nil
}

// startFakeObserver serves a fake WCN observer on a temp Unix socket returning
// a cleanup function and the socket address.
func startFakeObserver(t *testing.T, flows []*flowpb.Flow) (sock string, cleanup func()) {
	t.Helper()
	sock = filepath.Join(t.TempDir(), "obs.sock")
	lst, err := (&net.ListenConfig{}).Listen(context.Background(), "unix", sock)
	require.NoError(t, err)
	s := grpc.NewServer()
	observerv1.RegisterObserverServer(s, &fakeObserver{flows: flows})
	go func() { _ = s.Serve(lst) }()
	cleanup = func() { s.Stop() }
	return sock, cleanup
}

func TestObserverSourceStreamsFlows(t *testing.T) {
	if _, err := log.SetupZapLogger(log.GetDefaultLogOpts()); err != nil {
		t.Fatal(err)
	}
	fls := []*flowpb.Flow{
		{
			Time:    timestamppb.Now(),
			IP:      &flowpb.IP{Source: "10.0.0.1", Destination: "10.0.0.2"},
			Verdict: flowpb.Verdict_FORWARDED,
		},
	}
	sockPath, stop := startFakeObserver(t, fls)
	defer stop()

	src := newObserverSource(sockPath)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := src.Start(ctx)
	require.NoError(t, err)
	select {
	case ev := <-ch:
		require.NotNil(t, ev)
		require.NotNil(t, ev.Event)
	case <-time.After(3 * time.Second):
		t.Fatal("expected a flow from ObserverSource")
	}
	require.NoError(t, src.Stop())
}

func TestObserverSourceStartFailure(t *testing.T) {
	if _, err := log.SetupZapLogger(log.GetDefaultLogOpts()); err != nil {
		t.Fatal(err)
	}
	// A path with no listener should cause Start to fail to connect.
	src := newObserverSource(filepath.Join(t.TempDir(), "missing.sock"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err := src.Start(ctx)
	require.Error(t, err)
}
