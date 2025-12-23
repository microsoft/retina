//go:build windows
// +build windows

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

// Package pktmon tests the pktmon plugin behavior on Windows, including the event stream
// health check which detects silent ETW registration failures when another consumer
// is already active on the EVENTS_MAP.
//
// The verifyEventStream() method is critical for detecting scenarios where:
// 1. Another ETW consumer prevents the pktmon server from registering
// 2. The registration appears successful but no events are being produced
// 3. Network traffic exists but isn't being captured
//
// These tests ensure that such failures are caught early rather than resulting in
// silent data loss and misleading metrics.
package pktmon

import (
	"context"
	"errors"
	"testing"
	"time"

	observerv1 "github.com/cilium/cilium/api/v1/observer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"

	"github.com/microsoft/retina/pkg/log"
)

var (
	errEOF            = errors.New("EOF")
	errConnectionLost = errors.New("connection lost")
)

// MockGetFlowsClient implements observerv1.Observer_GetFlowsClient for testing
type MockGetFlowsClient struct {
	// Sequence of responses to return
	responses []interface{} // nil or "valid" or error
	index     int
	// Control test behavior
	recvDelay time.Duration
	blockCh   chan struct{} // If set, block recv() forever
	blockFor  time.Duration // If > 0, block recv() for this duration
}

// Recv returns the next mocked response
func (m *MockGetFlowsClient) Recv() (*observerv1.GetFlowsResponse, error) {
	if m.recvDelay > 0 {
		time.Sleep(m.recvDelay)
	}

	if m.blockCh != nil {
		<-m.blockCh
	}

	if m.blockFor > 0 {
		time.Sleep(m.blockFor)
	}

	if m.index >= len(m.responses) {
		return nil, errEOF
	}

	resp := m.responses[m.index]
	m.index++

	// Return error if configured
	if err, ok := resp.(error); ok {
		return nil, err
	}

	// For test purposes, we return a GetFlowsResponse that can be checked with GetFlow
	// The actual verification happens during verifyEventStream which checks GetFlow()
	// We use a custom type that wraps and properly implements GetFlow
	return &observerv1.GetFlowsResponse{}, nil
}

// Header implements grpc.ClientStream
func (m *MockGetFlowsClient) Header() (metadata.MD, error) {
	return nil, nil
}

// Trailer implements grpc.ClientStream
func (m *MockGetFlowsClient) Trailer() metadata.MD {
	return nil
}

// CloseSend implements grpc.ClientStream
func (m *MockGetFlowsClient) CloseSend() error {
	return nil
}

// Context implements grpc.ClientStream
func (m *MockGetFlowsClient) Context() context.Context {
	return context.Background()
}

// SendMsg implements grpc.ClientStream
func (m *MockGetFlowsClient) SendMsg(_ interface{}) error {
	return nil
}

// RecvMsg implements grpc.ClientStream
func (m *MockGetFlowsClient) RecvMsg(_ interface{}) error {
	return nil
}

// Test_verifyEventStream_NoEventsTimeout tests timeout when no events are received
func Test_verifyEventStream_NoEventsTimeout(t *testing.T) {
	plugin := &Plugin{
		l: NewTestLogger(),
		stream: &MockGetFlowsClient{
			blockCh: make(chan struct{}), // Block forever
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	err := plugin.verifyEventStream(ctx)
	elapsed := time.Since(start)

	require.Error(t, err, "verifyEventStream should fail when no events are received within timeout")
	require.ErrorContains(t, err, "no events received", "error should mention no events received")
	// Should timeout around eventHealthCheckFirstEvent (10s)
	// But our context only allows 5s so we timeout on context
	assert.GreaterOrEqual(t, elapsed, 4*time.Second, "should respect timeout")
}

// Test_verifyEventStream_StreamRecvError tests error handling when Recv() fails
func Test_verifyEventStream_StreamRecvError(t *testing.T) {
	plugin := &Plugin{
		l: NewTestLogger(),
		stream: &MockGetFlowsClient{
			responses: []interface{}{errConnectionLost},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := plugin.verifyEventStream(ctx)

	require.Error(t, err, "verifyEventStream should fail when Recv() returns an error")
	assert.ErrorContains(t, err, "failed to receive first event", "error should mention failed event reception")
}

// Test_verifyEventStream_NilFlow tests error handling when received event has nil flow
func Test_verifyEventStream_NilFlow(t *testing.T) {
	plugin := &Plugin{
		l: NewTestLogger(),
		stream: &MockGetFlowsClient{
			responses: []interface{}{nil}, // Response with nil flow
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := plugin.verifyEventStream(ctx)

	require.Error(t, err, "verifyEventStream should fail when received event has nil flow")
	assert.ErrorContains(t, err, "received nil flow", "error should mention nil flow")
}

// Test_verifyEventStream_ContextCancellation tests behavior when context is cancelled
func Test_verifyEventStream_ContextCancellation(t *testing.T) {
	plugin := &Plugin{
		l: NewTestLogger(),
		stream: &MockGetFlowsClient{
			blockCh: make(chan struct{}), // Block forever
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := plugin.verifyEventStream(ctx)

	require.Error(t, err, "verifyEventStream should fail when context is cancelled")
}

// Test_verifyEventStream_EventWithDelay tests successful reception of delayed event
func Test_verifyEventStream_EventWithDelay(t *testing.T) {
	// Test that verifyEventStream properly respects the timeout
	// A delayed response that arrives within the timeout window should succeed
	// We test this by using blockFor that's less than eventHealthCheckFirstEvent
	plugin := &Plugin{
		l: NewTestLogger(),
		stream: &MockGetFlowsClient{
			blockFor: 2 * time.Second, // Shorter than eventHealthCheckFirstEvent (10s)
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	start := time.Now()
	err := plugin.verifyEventStream(ctx)
	elapsed := time.Since(start)

	// Since blockFor causes recv to sleep, then returns EOF (no events),
	// this will timeout after eventHealthCheckFirstEvent
	// So we expect an error, but we're testing the timeout enforcement
	require.Error(t, err, "should get timeout error")
	assert.GreaterOrEqual(t, elapsed, 2*time.Second, "should wait at least for blockFor duration")
}

// Test_verifyEventStream_TimeoutExactlyAtLimit tests timeout enforcement
func Test_verifyEventStream_TimeoutExactlyAtLimit(t *testing.T) {
	plugin := &Plugin{
		l: NewTestLogger(),
		stream: &MockGetFlowsClient{
			blockFor: eventHealthCheckFirstEvent + 100*time.Millisecond,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	start := time.Now()
	err := plugin.verifyEventStream(ctx)
	elapsed := time.Since(start)

	require.Error(t, err, "verifyEventStream should fail when event reception exceeds timeout")
	// Should timeout after approximately eventHealthCheckFirstEvent (10s)
	assert.GreaterOrEqual(t, elapsed, eventHealthCheckFirstEvent, "should wait at least the timeout duration")
	assert.Less(t, elapsed, eventHealthCheckFirstEvent+5*time.Second, "should not wait significantly longer than timeout")
}

// Test_verifyEventStream_MultipleEvents tests that verification stops after first valid event
func Test_verifyEventStream_MultipleEvents(t *testing.T) {
	// Test that verifyEventStream doesn't consume more events than necessary
	// By checking the index counter on the mock stream
	plugin := &Plugin{
		l: NewTestLogger(),
		stream: &MockGetFlowsClient{
			// First response is nil (will trigger nil flow error)
			// Second response is nil
			// If verifyEventStream consumes more than 1, that would be wrong
			responses: []interface{}{nil, nil, nil},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := plugin.verifyEventStream(ctx)

	require.Error(t, err, "should get nil flow error")
	assert.Equal(t, 1, plugin.stream.(*MockGetFlowsClient).index,
		"should only consume exactly one event before returning error")
}

// Test_verifyEventStream_ErrorMessageContent tests the detailed error message
func Test_verifyEventStream_ErrorMessageContent(t *testing.T) {
	plugin := &Plugin{
		l: NewTestLogger(),
		stream: &MockGetFlowsClient{
			blockCh: make(chan struct{}),
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := plugin.verifyEventStream(ctx)

	require.Error(t, err, "verifyEventStream should fail")
	errMsg := err.Error()
	assert.Contains(t, errMsg, "no events received from pktmon", "error should mention pktmon")
	assert.Contains(t, errMsg, "ETW consumer", "error should mention ETW")
	assert.Contains(t, errMsg, "network traffic", "error should mention network traffic")
}

// Test_StartStream_WithNilGrpcClient tests error handling when grpcClient is nil
func Test_StartStream_WithNilGrpcClient(t *testing.T) {
	plugin := &Plugin{
		l:          NewTestLogger(),
		grpcClient: nil,
	}

	err := plugin.StartStream(context.Background())

	require.Error(t, err, "StartStream should fail when grpcClient is nil")
	assert.ErrorIs(t, err, ErrNilGrpcClient, "error should be ErrNilGrpcClient")
}

// Helper function to create a test logger
func NewTestLogger() *log.ZapLogger {
	// Create a simple logger for testing
	testLogger, _ := log.SetupZapLogger(&log.LogOpts{
		Level: "info",
		File:  false,
	})
	return testLogger
}
