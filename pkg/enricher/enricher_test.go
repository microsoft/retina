// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package enricher

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/cilium/cilium/api/v1/flow"
	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	"github.com/microsoft/retina/pkg/common"
	"github.com/microsoft/retina/pkg/controllers/cache"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/pubsub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestEnricherSecondaryIPs(t *testing.T) {
	evCount := 20
	// by design per ring, the last written item is not readable
	// since we have two rings here, there will be a diff of 2 items
	// between the input and output ring
	expectedOutputCount := 18

	opts := log.GetDefaultLogOpts()
	opts.Level = "debug"
	log.SetupZapLogger(opts)
	l := log.Logger().Named("test-enricher")

	ctx, cancel := context.WithCancel(context.Background())
	c := cache.New(pubsub.New())

	// construct the source endpoint
	sourceEndpoints := common.NewRetinaEndpoint("pod1", "ns1", &common.IPAddresses{
		IPv4:       net.IPv4(1, 1, 1, 1),
		OtherIPv4s: []net.IP{net.IPv4(1, 1, 1, 2)},
	})

	err := c.UpdateRetinaEndpoint(sourceEndpoints)
	require.NoError(t, err)

	// construct the destination endpoint
	destEndpoints := common.NewRetinaEndpoint("pod2", "ns2", &common.IPAddresses{
		IPv4:       net.IPv4(2, 2, 2, 2),
		OtherIPv4s: []net.IP{net.IPv4(2, 2, 2, 3)},
	})

	err = c.UpdateRetinaEndpoint(destEndpoints)
	require.NoError(t, err)

	// get the enricher
	e := New(ctx, c)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		for i := 0; i < evCount; i++ {
			// The Event Source IP is the secondary IP of the source endpoint
			// The Event Destination IP is the secondary IP of the destination endpoint
			addEvent(e, "1.1.1.2", "2.2.2.3")
		}
		wg.Done()
	}()

	e.Run()

	wg.Add(1)
	go func() {
		count := 0
		oreader := e.ExportReader()
		for {
			ev := oreader.NextFollow(ctx)
			if ev == nil {
				break
			}

			l.Info("One Received event", zap.Any("event", ev))
			// check whether the event is enriched correctly
			assertEqualEndpoint(t, sourceEndpoints, ev.Event.(*flow.Flow).GetSource())
			assertEqualEndpoint(t, destEndpoints, ev.Event.(*flow.Flow).GetDestination())
			count++
		}
		assert.Equal(t, expectedOutputCount, count, "one")
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		count := 0
		oreader := e.ExportReader()
		for {
			ev := oreader.NextFollow(ctx)
			if ev == nil {
				break
			}
			// check whether the event is enriched correctly
			assertEqualEndpoint(t, sourceEndpoints, ev.Event.(*flow.Flow).GetSource())
			assertEqualEndpoint(t, destEndpoints, ev.Event.(*flow.Flow).GetDestination())
			count++
		}
		assert.Equal(t, expectedOutputCount, count, "two")
		wg.Done()
	}()

	time.Sleep(3 * time.Second)
	cancel()
	wg.Wait()
}

func addEvent(e *Enricher, sourceIP, destIP string) {
	l := log.Logger().Named("addev")
	ev := &v1.Event{
		Timestamp: timestamppb.Now(),
		Event: &flow.Flow{
			IP: &flow.IP{
				IpVersion:   1,
				Source:      sourceIP,
				Destination: destIP,
			},
		},
	}

	l.Info("Adding event", zap.Any("event", ev))
	time.Sleep(100 * time.Millisecond)
	e.Write(ev)
}

func assertEqualEndpoint(t *testing.T, expected *common.RetinaEndpoint, actual *flow.Endpoint) {
	assert.Equal(t, expected.Namespace(), actual.GetNamespace())
	assert.Equal(t, expected.Name(), actual.GetPodName())
}
