// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
// nolint

package main

import (
	"context"

	"github.com/cilium/cilium/api/v1/flow"
	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	"github.com/microsoft/retina/pkg/controllers/cache"
	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/enricher/base"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/pubsub"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func main() {
	opts := log.GetDefaultLogOpts()
	opts.Level = "debug"
	log.SetupZapLogger(opts)
	l := log.Logger().Named("test-enricher")

	ctx := context.Background()
	c := cache.New(pubsub.New())

	e := enricher.NewStandard(ctx, c)

	e.Run()

	for i := 0; i < 10; i++ {
		addEvent(e)
	}

	oreader := e.ExportReader()

	for i := 0; i < 10; i++ {
		l.Info("Receiving event")
		ev := oreader.NextFollow(ctx)

		l.Info("Received event", zap.Any("event", ev))
	}
}

func addEvent(e base.EnricherInterface) {
	l := log.Logger().Named("addev")
	ev := &v1.Event{
		Timestamp: timestamppb.Now(),
		Event:     &flow.Flow{},
	}

	l.Info("Adding event", zap.Any("event", ev))
	e.Write(ev)
}
