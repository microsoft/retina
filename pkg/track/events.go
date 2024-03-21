// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package track

import (
	"context"
	"sync"

	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/log"
)

type Track struct {
	ctx             context.Context
	cancel          context.CancelFunc
	externalChannel chan *v1.Event
	l               *log.ZapLogger
	eh              enricher.EnricherInterface
}

const (
	DefaultExternalChannelSize = 1000
)

var (
	once sync.Once
	t    *Track
)

func New() *Track {
	once.Do(func() {
		var e *enricher.Enricher
		if e = enricher.Instance(); e == nil {
			// Should never happen, but log it just in case.
			t.l.Error("enricher instance not found, track is useless")
			return
		}

		t = &Track{
			externalChannel: make(chan *v1.Event, DefaultExternalChannelSize),
			l:               log.Logger().Named("track-events"),
			eh:              e,
		}
	})
	return t
}

// Allow invoker to connect the channel to a source of events.
func (t *Track) Channel() chan *v1.Event {
	return t.externalChannel
}

// Start is a blocking function that listens for events on the external channel.
func (t *Track) Start(ctx context.Context) {
	t.l.Info("Started tracking events")
	t.ctx, t.cancel = context.WithCancel(ctx)
	for {
		select {
		case <-t.ctx.Done():
			t.l.Info("context cancelled, stopping track")
			return
		case ev := <-t.externalChannel:
			t.eh.Write(ev) // Write to the enricher's input ring.
		}
	}
}

func (t *Track) Stop() {
	t.cancel()
	close(t.externalChannel)
	t.l.Info("Stopped tracking events")
}
