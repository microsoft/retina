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

var (
	// number of events
	eventsGeneratedCount = 5

	// construct the endpoints
	sourcePod = common.NewRetinaEndpoint("pod1", "ns1", &common.IPAddresses{
		IPv4:       net.IPv4(1, 1, 1, 1),
		OtherIPv4s: []net.IP{net.IPv4(1, 1, 1, 2)},
	})
	destPod = common.NewRetinaEndpoint("pod2", "ns2", &common.IPAddresses{
		IPv4:       net.IPv4(2, 2, 2, 2),
		OtherIPv4s: []net.IP{net.IPv4(2, 2, 2, 3)},
	})
	// sourceIP = sourcePod.NetIPs().IPv4.String()
	// destIP   = destPod.NetIPs().IPv4.String()
	sourceIP = sourcePod.NetIPs().OtherIPv4s[0].String()
	destIP   = destPod.NetIPs().OtherIPv4s[0].String()

	// construct events
	normal = &v1.Event{
		Timestamp: timestamppb.Now(),
		Event: &flow.Flow{
			Time: timestamppb.Now(),
			IP: &flow.IP{
				IpVersion:   1,
				Source:      sourceIP,
				Destination: destIP,
			},
		},
	}
	nilFlow = &v1.Event{
		Timestamp: timestamppb.Now(),
		Event:     nil,
	}
	nilIP = &v1.Event{
		Timestamp: timestamppb.Now(),
		Event: &flow.Flow{
			Time: timestamppb.Now(),
			IP:   nil,
		},
	}
	events = []*v1.Event{
		normal, nilFlow, nilIP,
	}
)

func writeEventToEnricher(t *testing.T, e *Enricher, ev *v1.Event) {
	t.Helper()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		for range eventsGeneratedCount {
			l := log.Logger().Named("addev")
			l.Info("Adding event", zap.Any("event", ev))
			time.Sleep(100 * time.Millisecond)
			e.Write(ev)
		}
		wg.Done()
	}()
	e.Run()
	wg.Wait()
}

func TestEnricher(t *testing.T) {
	opts := log.GetDefaultLogOpts()
	opts.Level = "debug"
	log.SetupZapLogger(opts)

	c := cache.New(pubsub.New())

	err := c.UpdateRetinaEndpoint(sourcePod)
	require.NoError(t, err)

	err = c.UpdateRetinaEndpoint(destPod)
	require.NoError(t, err)

	e := new(context.Background(), c)

	var wg sync.WaitGroup
	defer wg.Wait()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, ev := range events {
		writeEventToEnricher(t, e, ev)
	}

	l := log.Logger().Named("test-enricher")

	l.Info("Starting to read from enricher")
	wg.Add(1)
	go func() {
		oreader := e.ExportReader()
		for {
			ev := oreader.NextFollow(ctx)
			if ev == nil {
				l.Info("No more events to read from enricher")
				break
			}

			l.Info("One Received event", zap.Any("event", ev))
			assertEqualEndpoint(t, sourcePod, ev.Event.(*flow.Flow).GetSource())
			assertEqualEndpoint(t, destPod, ev.Event.(*flow.Flow).GetDestination())
		}
		wg.Done()
	}()

	time.Sleep(3 * time.Second)

}

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

	err := c.UpdateRetinaEndpoint(sourcePod)
	require.NoError(t, err)

	err = c.UpdateRetinaEndpoint(destPod)
	require.NoError(t, err)

	// create new enricher (not using singleton here)
	e := new(ctx, c)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		for i := 0; i < evCount; i++ {
			// The Event Source IP is the secondary IP of the source endpoint
			secondarySourceIP := sourcePod.NetIPs().OtherIPv4s[0].String()
			// The Event Destination IP is the secondary IP of the destination endpoint
			secondaryDestIP := destPod.NetIPs().OtherIPv4s[0].String()

			addEvent(e, secondarySourceIP, secondaryDestIP)
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
			assertEqualEndpoint(t, sourcePod, ev.Event.(*flow.Flow).GetSource())
			assertEqualEndpoint(t, destPod, ev.Event.(*flow.Flow).GetDestination())
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
			assertEqualEndpoint(t, sourcePod, ev.Event.(*flow.Flow).GetSource())
			assertEqualEndpoint(t, destPod, ev.Event.(*flow.Flow).GetDestination())
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
