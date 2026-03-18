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
	retinav1alpha1 "github.com/microsoft/retina/crd/api/v1alpha1"
	"github.com/microsoft/retina/pkg/common"
	"github.com/microsoft/retina/pkg/controllers/cache"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/pubsub"
	"github.com/microsoft/retina/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	_, err := log.SetupZapLogger(opts)
	require.NoError(t, err)

	c := cache.New(pubsub.New())

	err = c.UpdateRetinaEndpoint(sourcePod)
	require.NoError(t, err)

	err = c.UpdateRetinaEndpoint(destPod)
	require.NoError(t, err)

	e := newEnricher(context.Background(), c)

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
	e := newEnricher(ctx, c)
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

func TestEnricherZoneResolution(t *testing.T) {
	opts := log.GetDefaultLogOpts()
	opts.Level = "debug"
	log.SetupZapLogger(opts)

	c := cache.New(pubsub.New())

	// Add a node with a zone to the cache.
	node := common.NewRetinaNode("node-1", net.IPv4(10, 0, 0, 100), "zone-1")
	require.NoError(t, c.UpdateRetinaNode(node))

	// Create endpoints with nodeIP pointing to the node above.
	srcEp := common.RetinaEndpointCommonFromAPI(&retinav1alpha1.RetinaEndpoint{
		ObjectMeta: metav1.ObjectMeta{Name: "src-pod", Namespace: "ns1"},
		Spec: retinav1alpha1.RetinaEndpointSpec{
			PodIP:  "1.1.1.1",
			PodIPs: []string{"1.1.1.1"},
			NodeIP: "10.0.0.100",
		},
	})
	dstEp := common.RetinaEndpointCommonFromAPI(&retinav1alpha1.RetinaEndpoint{
		ObjectMeta: metav1.ObjectMeta{Name: "dst-pod", Namespace: "ns2"},
		Spec: retinav1alpha1.RetinaEndpointSpec{
			PodIP:  "2.2.2.2",
			PodIPs: []string{"2.2.2.2"},
			NodeIP: "10.0.0.100",
		},
	})

	require.NoError(t, c.UpdateRetinaEndpoint(srcEp))
	require.NoError(t, c.UpdateRetinaEndpoint(dstEp))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	e := newEnricher(ctx, c)

	// Get the export reader before running and writing, so we don't miss events.
	oreader := e.ExportReader()
	e.Run()

	// Write multiple events to ensure at least one makes it through the ring.
	for range 3 {
		e.Write(&v1.Event{
			Timestamp: timestamppb.Now(),
			Event: &flow.Flow{
				IP: &flow.IP{
					IpVersion:   1,
					Source:      "1.1.1.1",
					Destination: "2.2.2.2",
				},
			},
		})
		time.Sleep(100 * time.Millisecond)
	}

	enrichedEv := oreader.NextFollow(ctx)
	require.NotNil(t, enrichedEv)

	enrichedFlow := enrichedEv.Event.(*flow.Flow)
	assert.Equal(t, "zone-1", utils.SourceZone(enrichedFlow))
	assert.Equal(t, "zone-1", utils.DestinationZone(enrichedFlow))
}

func TestEnricherZoneResolution_NoNode(t *testing.T) {
	opts := log.GetDefaultLogOpts()
	opts.Level = "debug"
	log.SetupZapLogger(opts)

	c := cache.New(pubsub.New())

	// Create endpoint with nodeIP but no node in cache.
	ep := common.RetinaEndpointCommonFromAPI(&retinav1alpha1.RetinaEndpoint{
		ObjectMeta: metav1.ObjectMeta{Name: "pod1", Namespace: "ns1"},
		Spec: retinav1alpha1.RetinaEndpointSpec{
			PodIP:  "3.3.3.3",
			PodIPs: []string{"3.3.3.3"},
			NodeIP: "10.0.0.200",
		},
	})
	require.NoError(t, c.UpdateRetinaEndpoint(ep))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	e := newEnricher(ctx, c)

	oreader := e.ExportReader()
	e.Run()

	for range 3 {
		e.Write(&v1.Event{
			Timestamp: timestamppb.Now(),
			Event: &flow.Flow{
				IP: &flow.IP{
					IpVersion:   1,
					Source:      "3.3.3.3",
					Destination: "9.9.9.9",
				},
			},
		})
		time.Sleep(100 * time.Millisecond)
	}

	enrichedEv := oreader.NextFollow(ctx)
	require.NotNil(t, enrichedEv)

	enrichedFlow := enrichedEv.Event.(*flow.Flow)
	// Node not in cache, so zone should be "unknown".
	assert.Equal(t, "unknown", utils.SourceZone(enrichedFlow))
	assert.Equal(t, "unknown", utils.DestinationZone(enrichedFlow))
}
