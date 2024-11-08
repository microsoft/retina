// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package pubsub

type CallBackFunc func(interface{})

//go:generate mockgen -destination=mock_pubsubinterface.go  -copyright_file=../lib/ignore_headers.txt -package=pubsub github.com/microsoft/retina/pkg/pubsub PubSubInterface

// this file defines the interface a simple pubsub implementation should implement
type PubSubInterface interface {
	// Publish publishes the given message to the given topic
	Publish(topic PubSubTopic, msg interface{})
	// Subscribe subscribes to the given topic and calls the given callback function
	// when a message is published to the topic
	Subscribe(topic PubSubTopic, callback *CallBackFunc) string
	// Unsubscribe unsubscribes from the given topic
	Unsubscribe(topic PubSubTopic, uuid string) error
}

type PubSubTopic string
