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
	// Can be: nil (nil flow), "valid" (valid flow), error, or *observerv1.Flow (explicit flow)
	responses []interface{}
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

	// Return response with non-nil flow if configured
	if flow, ok := resp.(*observerv1.Flow); ok {
		return &observerv1.GetFlowsResponse{ResponseTypes: &observerv1.GetFlowsResponse_Flow{Flow: flow}}, nil
	}

	// Handle "valid" string marker - return a valid flow for testing
	if marker, ok := resp.(string); ok && marker == "valid" {
		validFlow := &observerv1.Flow{
			Source:      &observerv1.Endpoint{PodName: "test-src"},
			Destination: &observerv1.Endpoint{PodName: "test-dst"},
		}
		return &observerv1.GetFlowsResponse{ResponseTypes: &observerv1.GetFlowsResponse_Flow{Flow: validFlow}}, nil
	}

	// For test purposes, return empty GetFlowsResponse with nil Flow
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
// With the new behavior, no events (due to lack of traffic) should return nil,
// not an error, since the stream connection itself is healthy.
// Note: verifyEventStream() uses an independent 60-second internal timeout,
// so the test context parameter is not used for timeout control.
func Test_verifyEventStream_NoEventsTimeout(t *testing.T) {
	plugin := &Plugin{
		l: NewTestLogger(),
		stream: &MockGetFlowsClient{
			blockCh: make(chan struct{}), // Block forever - will timeout internally
		},
	}

	start := time.Now()
	err := plugin.verifyEventStream()
	elapsed := time.Since(start)

	require.NoError(t, err, "verifyEventStream should return nil when no events are received (timeout due to no traffic)")
	// Should timeout after approximately eventHealthCheckFirstEvent (60s)
	assert.GreaterOrEqual(t, elapsed, 59*time.Second, "should wait close to the 60s health check timeout")
	assert.Less(t, elapsed, 62*time.Second, "should not wait significantly longer than the 60s timeout")
}

// Test_verifyEventStream_StreamRecvError tests error handling when Recv() fails
// Actual connection errors (not timeouts) should still fail
func Test_verifyEventStream_StreamRecvError(t *testing.T) {
	plugin := &Plugin{
		l: NewTestLogger(),
		stream: &MockGetFlowsClient{
			responses: []interface{}{errConnectionLost},
		},
	}

	err := plugin.verifyEventStream()

	require.Error(t, err, "verifyEventStream should fail when Recv() returns an error")
}

// Test_verifyEventStream_NilFlow tests that occasional nil flows are skipped
// The health check should continue waiting for valid flows rather than failing immediately
func Test_verifyEventStream_NilFlow(t *testing.T) {
	plugin := &Plugin{
		l: NewTestLogger(),
		stream: &MockGetFlowsClient{
			// Start with nil flow, then valid flow
			responses: []interface{}{nil, "valid"},
		},
	}

	err := plugin.verifyEventStream()

	require.NoError(t, err, "verifyEventStream should skip occasional nil flows and succeed when valid flow arrives")
	assert.Equal(t, 2, plugin.stream.(*MockGetFlowsClient).index,
		"should consume both nil and valid flow")
}

// Test_verifyEventStream_TooManyNilFlows tests that persistent nil flows are detected as proto mismatch
func Test_verifyEventStream_TooManyNilFlows(t *testing.T) {
	plugin := &Plugin{
		l: NewTestLogger(),
		stream: &MockGetFlowsClient{
			// More nil flows than allowed
			responses: make([]interface{}, maxNilFlowsAllowed+2),
		},
	}

	err := plugin.verifyEventStream()

	require.Error(t, err, "verifyEventStream should fail when too many nil flows are received")
	assert.ErrorContains(t, err, "too many nil flows", "error should mention too many nil flows")
}

// Test_verifyEventStream_EventWithDelay tests successful reception of delayed non-nil flow
// This verifies that the health check properly waits for a valid flow even with delay
func Test_verifyEventStream_EventWithDelay(t *testing.T) {
	// Create a valid flow with basic structure
	validFlow := &observerv1.Flow{
		Source:      &observerv1.Endpoint{PodName: "pod-1", Namespace: "default"},
		Destination: &observerv1.Endpoint{PodName: "pod-2", Namespace: "default"},
	}

	plugin := &Plugin{
		l: NewTestLogger(),
		stream: &MockGetFlowsClient{
			blockFor:  2 * time.Second,          // Delay before first message
			responses: []interface{}{validFlow}, // Then send valid non-nil flow
		},
	}

	start := time.Now()
	err := plugin.verifyEventStream()
	elapsed := time.Since(start)

	require.NoError(t, err, "should succeed when valid non-nil flow eventually arrives")
	assert.GreaterOrEqual(t, elapsed, 1900*time.Millisecond, "should wait at least 1.9s for blockFor delay")
	assert.Less(t, elapsed, 3*time.Second, "should not wait much longer than the 2s blockFor")
}

// Test_verifyEventStream_TimeoutExactlyAtLimit tests timeout enforcement
// When the health check timeout occurs (no traffic), it should return nil (success)
// Note: verifyEventStream() uses an independent 60-second internal context
func Test_verifyEventStream_TimeoutExactlyAtLimit(t *testing.T) {
	plugin := &Plugin{
		l: NewTestLogger(),
		stream: &MockGetFlowsClient{
			blockFor: eventHealthCheckFirstEvent + 100*time.Millisecond,
		},
	}

	start := time.Now()
	err := plugin.verifyEventStream()
	elapsed := time.Since(start)

	require.NoError(t, err, "verifyEventStream should return nil when health check timeout occurs (no traffic)")
	// Should timeout after approximately eventHealthCheckFirstEvent (60s)
	expectedMin := eventHealthCheckFirstEvent - 500*time.Millisecond
	expectedMax := eventHealthCheckFirstEvent + 2*time.Second
	assert.GreaterOrEqual(t, elapsed, expectedMin, "should wait at least close to the health check timeout")
	assert.Less(t, elapsed, expectedMax, "should not wait significantly longer than the health check timeout")
}

// Test_verifyEventStream_MultipleEvents tests that verification succeeds with valid flows
func Test_verifyEventStream_MultipleEvents(t *testing.T) {
	// Test that verifyEventStream succeeds when valid flows are available
	validFlow := &observerv1.Flow{
		Source:      &observerv1.Endpoint{PodName: "source", Namespace: "default"},
		Destination: &observerv1.Endpoint{PodName: "destination", Namespace: "default"},
	}

	plugin := &Plugin{
		l: NewTestLogger(),
		stream: &MockGetFlowsClient{
			// Multiple valid flows
			responses: []interface{}{validFlow, validFlow, validFlow},
		},
	}

	err := plugin.verifyEventStream()

	require.NoError(t, err, "should succeed when valid flows are available")
}

// Test_verifyEventStream_NoTrafficLogsWarning tests that no-traffic timeout returns success
// Note: verifyEventStream() uses an independent 60-second internal context
func Test_verifyEventStream_NoTrafficLogsWarning(t *testing.T) {
	plugin := &Plugin{
		l: NewTestLogger(),
		stream: &MockGetFlowsClient{
			blockCh: make(chan struct{}), // Block forever - will timeout internally
		},
	}

	err := plugin.verifyEventStream()

	require.NoError(t, err, "verifyEventStream should succeed when timeout occurs (no traffic scenario)")
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
