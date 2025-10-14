// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build windows

package monitoragent

import (
	"context"
	"testing"

	"github.com/cilium/cilium/pkg/monitor/agent/consumer"
	"github.com/cilium/cilium/pkg/monitor/agent/listener"
)

func TestAttachToEventsMapReturnsError(t *testing.T) {
	// Create a new monitor agent
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agent := &monitorAgent{
		ctx:              ctx,
		listeners:        make(map[listener.MonitorListener]struct{}),
		consumers:        make(map[consumer.MonitorConsumer]struct{}),
		perfReaderCancel: func() {},
	}

	// AttachToEventsMap should return an error on Windows
	// since only one consumer can attach at a time
	err := agent.AttachToEventsMap(0)
	if err == nil {
		t.Error("AttachToEventsMap should return an error on Windows")
	}
	if err != ErrEventsMapAttachFailed {
		t.Errorf("Expected ErrEventsMapAttachFailed, got %v", err)
	}
}

func TestIsCtxDone(t *testing.T) {
	// Test with active context
	ctx, cancel := context.WithCancel(context.Background())
	if isCtxDone(ctx) {
		t.Error("Context should not be done")
	}

	// Test with cancelled context
	cancel()
	if !isCtxDone(ctx) {
		t.Error("Context should be done after cancel")
	}
}

// mockConsumer implements the MonitorConsumer interface for testing
type mockConsumer struct{}

func (m *mockConsumer) NotifyAgentEvent(typ int, message interface{}) {}

func TestRegisterConsumerWithCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	agent := &monitorAgent{
		ctx:              ctx,
		listeners:        make(map[listener.MonitorListener]struct{}),
		consumers:        make(map[consumer.MonitorConsumer]struct{}),
		perfReaderCancel: func() {},
	}

	// Create a mock consumer
	mc := &mockConsumer{}

	// Try to register the consumer with a cancelled context
	agent.RegisterNewConsumer(mc)

	// Should not register when context is cancelled
	agent.Lock()
	hasSubscribers := len(agent.consumers) > 0
	agent.Unlock()

	if hasSubscribers {
		t.Error("Should not register consumer when context is cancelled")
	}
}
