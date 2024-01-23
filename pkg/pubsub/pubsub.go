// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package pubsub

import (
	"fmt"
	"sync"

	"github.com/microsoft/retina/pkg/log"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

var (
	p    *PubSub
	once sync.Once
)

type PubSub struct {
	sync.RWMutex
	// l is the logger.
	l *log.ZapLogger
	// topicToCallback is a map of topic to a map of callback function
	topicToCallback map[PubSubTopic]map[string]*CallBackFunc
}

// New returns a new instance of PubSub.
func New() *PubSub {
	once.Do(func() {
		p = &PubSub{
			l:               log.Logger().Named(string("PubSub")),
			topicToCallback: make(map[PubSubTopic]map[string]*CallBackFunc),
		}
	})

	return p
}

// Publish publishes the given message to the given topic,
// and calls all the callback functions subscribed to the topic.
func (p *PubSub) Publish(topic PubSubTopic, msg interface{}) {
	p.RLock()
	defer p.RUnlock()

	// If there are no callbacks for the given topic, return nil.
	if _, ok := p.topicToCallback[topic]; !ok {
		p.l.Debug("no callbacks for topic", zap.String("topic", string(topic)))
		return
	}

	// Run all callbacks in parallel using a wait group.
	for uuid, callback := range p.topicToCallback[topic] {
		callback := callback
		p.l.Debug("running callback", zap.String("topic", string(topic)), zap.String("uuid", uuid))
		go func() {
			(*callback)(msg)
		}()
	}
}

// Subscribe subscribes to the given topic and calls the given callback function
// when a message is published to the topic, it returns a new uuid of the callback.
func (p *PubSub) Subscribe(topic PubSubTopic, callback *CallBackFunc) string {
	p.Lock()
	defer p.Unlock()

	// If the topic does not exist, create it.
	if _, ok := p.topicToCallback[topic]; !ok {
		p.topicToCallback[topic] = make(map[string]*CallBackFunc)
	}

	// Generate a new uuid for the callback.
	uuid := uuid.New().String()
	// Add the callback to the topic.
	p.topicToCallback[topic][uuid] = callback
	p.l.Debug("subscribed to topic", zap.String("topic", string(topic)), zap.String("uuid", uuid))

	return uuid
}

// Unsubscribe unsubscribes from the given topic.
func (p *PubSub) Unsubscribe(topic PubSubTopic, uuid string) error {
	p.Lock()
	defer p.Unlock()

	if uuid == "" {
		return fmt.Errorf("uuid cannot be empty")
	}

	// If the topic does not exist, return nil.
	if _, ok := p.topicToCallback[topic]; !ok {
		p.l.Debug("no callbacks for topic", zap.String("topic", string(topic)))
		return nil
	}

	// If the callback does not exist, return nil.
	if _, ok := p.topicToCallback[topic][uuid]; !ok {
		p.l.Debug("callback does not exist", zap.String("topic", string(topic)), zap.String("uuid", uuid))
		return nil
	}

	// Delete the callback from the topic.
	delete(p.topicToCallback[topic], uuid)
	p.l.Debug("unsubscribed from topic", zap.String("topic", string(topic)), zap.String("uuid", uuid))

	// Delete the topic if there are no callbacks left.
	if len(p.topicToCallback[topic]) == 0 {
		delete(p.topicToCallback, topic)
		p.l.Debug("deleted topic", zap.String("topic", string(topic)))
	}

	return nil
}
