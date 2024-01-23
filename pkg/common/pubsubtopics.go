// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package common

import "github.com/microsoft/retina/pkg/pubsub"

const (
	// PubSubPods topic
	PubSubPods pubsub.PubSubTopic = "pods"
	// PubSubEndpoints topic
	PubSubEndpoints pubsub.PubSubTopic = "endpoints"
	// PubSubSvc topic
	PubSubSvc pubsub.PubSubTopic = "svc"
	// PubSubNode topic
	PubSubNode pubsub.PubSubTopic = "node"
	// PubSubVeth topic
	PubSubVeth pubsub.PubSubTopic = "veth"
	// PubSubNamespace topic
	PubSubNamespace pubsub.PubSubTopic = "namespace"
	// PubSubFilterRule topic
	PubSubFilterRule pubsub.PubSubTopic = "filterrule"
	// PubSubAPIServer topic
	PubSubAPIServer pubsub.PubSubTopic = "apiserver"
)
