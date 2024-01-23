// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package pubsub

import (
	"testing"
	"time"

	"github.com/microsoft/retina/pkg/log"
	"github.com/stretchr/testify/assert"
)

const (
	until = 1 * time.Millisecond
)

func TestNewPubSub(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	p := New()
	assert.NotNil(t, p)
}

func TestPublish(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	p := New()
	p.Publish("topic", "msg")
}

func TestSubscribe(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	p := New()
	cb := CallBackFunc(func(msg interface{}) {})

	uuid := p.Subscribe("topic", &cb)
	assert.NotEmpty(t, uuid)
}

func TestUnsubscribe(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	p := New()
	cb := CallBackFunc(func(msg interface{}) {})

	uuid := p.Subscribe("topic", &cb)
	err := p.Unsubscribe("topic", uuid)
	assert.NoError(t, err)
}

func TestMultipleSubscribe(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())

	cb := CallBackFunc(func(msg interface{}) {})

	p := New()
	uuid1 := p.Subscribe("topic", &cb)
	uuid2 := p.Subscribe("topic", &cb)
	assert.NotEmpty(t, uuid1)
	assert.NotEmpty(t, uuid2)
}

func TestPubSub(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	ps := New()

	// Publisher 1 publishes a message to topic "topic1"
	ps.Publish("topic1", "Hello from Publisher 1!")

	// Publisher 2 publishes a message to topic "topic2"
	ps.Publish("topic2", "Hello from Publisher 2!")

	cb1 := CallBackFunc(func(msg interface{}) {
		if msg.(string) != "Hello from Publisher 1!" {
			t.Errorf("Expected 'Hello from Publisher 1!', got %s", msg)
		}
	})

	cb2 := CallBackFunc(func(msg interface{}) {
		if msg.(string) != "Hello from Publisher 2!" {
			t.Errorf("Expected 'Hello from Publisher 2!', got %s", msg)
		}
	})

	// Subscriber 1 subscribes to topic "topic1"
	subID1 := ps.Subscribe("topic1", &cb1)
	defer func() {
		// Unsubscribe Subscriber 1
		err := ps.Unsubscribe("topic1", subID1)
		if err != nil {
			t.Errorf("Failed to unsubscribe: %v", err)
		}
	}()

	// Subscriber 2 subscribes to topic "topic2"
	subID2 := ps.Subscribe("topic2", &cb2)
	defer func() {
		// Unsubscribe Subscriber 2
		err := ps.Unsubscribe("topic2", subID2)
		if err != nil {
			t.Errorf("Failed to unsubscribe: %v", err)
		}
	}()

	// Publisher 1 publishes another message to topic "topic1"
	ps.Publish("topic1", "Hello from Publisher 1!")

	// Publisher 2 publishes another message to topic "topic2"
	ps.Publish("topic2", "Hello from Publisher 2!")

	err := ps.Unsubscribe("topic1", "randomid")
	if err != nil {
		t.Errorf("Failed to unsubscribe: %v", err)
	}

	time.Sleep(until)
}
