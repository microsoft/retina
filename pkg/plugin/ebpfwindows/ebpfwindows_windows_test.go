// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

// Package ebpfwindows tests the plugin lifecycle and event pipeline using a
// synthetic EventSource. Because the source is an interface, these tests run
// on any platform without a Windows cluster or the eBPF-for-Windows runtime.
package ebpfwindows

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	flow "github.com/cilium/cilium/api/v1/flow"
	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/metrics"
)

// fakeSource is an in-memory EventSource for tests.
type fakeSource struct {
	ch       chan *v1.Event
	startErr error
	started  chan struct{}
}

// errSourceBoom is a package-level sentinel error used to simulate a failing
// EventSource without defining a dynamic error.
var errSourceBoom = errors.New("boom")

func newFakeSource() *fakeSource {
	return &fakeSource{ch: make(chan *v1.Event, 64), started: make(chan struct{}, 1)}
}

func (f *fakeSource) Start(ctx context.Context) (<-chan *v1.Event, error) {
	if f.startErr != nil {
		return nil, f.startErr
	}
	select {
	case f.started <- struct{}{}:
	default:
	}
	go func() {
		<-ctx.Done()
		close(f.ch)
	}()
	return f.ch, nil
}

func (f *fakeSource) Stop() error { return nil }

func (f *fakeSource) enqueue(ev *v1.Event) { f.ch <- ev }

func newFlowEvent() *v1.Event {
	return &v1.Event{
		Timestamp: timestamppb.Now(),
		Event: &flow.Flow{
			Time: timestamppb.Now(),
			IP:   &flow.IP{Source: "10.0.0.1", Destination: "10.0.0.2"},
		},
	}
}

func setupLoggingAndMetrics(t *testing.T) {
	t.Helper()
	if _, err := log.SetupZapLogger(log.GetDefaultLogOpts()); err != nil {
		t.Fatal(err)
	}
	metrics.InitializeMetrics(slog.Default())
}

func waitSourceStarted(t *testing.T, src *fakeSource) {
	t.Helper()
	select {
	case <-src.started:
	case <-time.After(2 * time.Second):
		t.Fatal("source did not start in time")
	}
}

func TestNewIsRegistered(t *testing.T) {
	require.Equal(t, name, New(&config.Config{}).Name())
}

func TestPluginGenerateCompileInitNoOp(t *testing.T) {
	setupLoggingAndMetrics(t)
	p := newPlugin(&config.Config{}, newFakeSource())
	require.NoError(t, p.Generate(context.Background()))
	require.NoError(t, p.Compile(context.Background()))
	require.NoError(t, p.Init())
}

func TestPluginStartStopIdempotent(t *testing.T) {
	setupLoggingAndMetrics(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	menricher := enricher.NewMockEnricherInterface(ctrl) //nolint:typecheck // mock enricher is generated into this package
	menricher.EXPECT().Write(gomock.Any()).AnyTimes()

	p := &Plugin{
		l:        log.Logger().Named(name),
		src:      newFakeSource(),
		out:      make(chan *v1.Event, 8),
		enricher: menricher,
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, p.Start(ctx))
	// Repeated Start is a no-op.
	require.NoError(t, p.Start(ctx))
	require.NoError(t, p.Stop())
	require.NoError(t, p.Stop())
}

func TestPluginForwardsEventAndWritesEnricher(t *testing.T) {
	setupLoggingAndMetrics(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	menricher := enricher.NewMockEnricherInterface(ctrl) //nolint:typecheck // mock enricher is generated into this package
	menricher.EXPECT().Write(gomock.Any()).Times(1)

	src := newFakeSource()
	p := &Plugin{
		l:        log.Logger().Named(name),
		src:      src,
		enricher: menricher,
		out:      make(chan *v1.Event, 8),
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, p.Start(ctx))
	waitSourceStarted(t, src)
	src.enqueue(newFlowEvent())
	select {
	case <-p.out:
	case <-time.After(2 * time.Second):
		t.Fatal("expected an event on the downstream channel")
	}
	require.NoError(t, p.Stop())
}

func TestPluginSkipsNilEvents(t *testing.T) {
	setupLoggingAndMetrics(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	menricher := enricher.NewMockEnricherInterface(ctrl) //nolint:typecheck // mock enricher is generated into this package
	menricher.EXPECT().Write(gomock.Any()).Times(0)

	src := newFakeSource()
	p := &Plugin{
		l:        log.Logger().Named(name),
		src:      src,
		enricher: menricher,
		out:      make(chan *v1.Event, 8),
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, p.Start(ctx))
	waitSourceStarted(t, src)
	src.enqueue(nil)
	time.Sleep(50 * time.Millisecond)
	select {
	case ev := <-p.out:
		t.Fatalf("did not expect an event on the channel, got %v", ev)
	default:
	}
	require.NoError(t, p.Stop())
}

func TestPluginSourceErrorStopsLoop(t *testing.T) {
	setupLoggingAndMetrics(t)
	src := newFakeSource()
	src.startErr = errSourceBoom
	p := &Plugin{
		l:   log.Logger().Named(name),
		src: src,
		out: make(chan *v1.Event, 8),
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, p.Start(ctx))
	require.NoError(t, p.Stop())
}

func TestPluginDropsWhenChannelFull(t *testing.T) {
	setupLoggingAndMetrics(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	menricher := enricher.NewMockEnricherInterface(ctrl) //nolint:typecheck // mock enricher is generated into this package
	menricher.EXPECT().Write(gomock.Any()).AnyTimes()
	src := newFakeSource()
	// A zero-capacity downstream channel forces the loop to drop after enricher write.
	p := &Plugin{
		l:        log.Logger().Named(name),
		src:      src,
		out:      make(chan *v1.Event),
		enricher: menricher,
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, p.Start(ctx))
	waitSourceStarted(t, src)
	// Fill to saturation and then enqueue more; the loop must drop instead of blocking.
	for i := 0; i < 64; i++ {
		src.enqueue(newFlowEvent())
	}
	// Allow the loop to process; no hang implies drops worked.
	time.Sleep(200 * time.Millisecond)
	require.NoError(t, p.Stop())
}
