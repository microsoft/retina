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
	"github.com/microsoft/retina/pkg/controllers/cache/standalone"
	"github.com/microsoft/retina/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnricherStandaloneWithEndpointPresent(t *testing.T) {
	opts := log.GetDefaultLogOpts()
	opts.Level = "debug"
	if _, err := log.SetupZapLogger(opts); err != nil {
		t.Errorf("Error setting up logger: %s", err)
	}

	eventCount := 20
	expectedOutputCount := eventCount - 2 // last written event is not readable due to ring buffers

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	testCache := standalone.New()
	sourceIP := "1.1.1.1"

	// Add endpoint to cache
	endpoint := common.NewRetinaEndpoint("pod1", "ns1", &common.IPAddresses{IPv4: net.ParseIP(sourceIP)})
	require.NoError(t, testCache.UpdateRetinaEndpoint(endpoint))

	// Create the enricher with standalone enabled
	enricher := newStandalone(ctx, testCache)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < eventCount; i++ {
			ev := &v1.Event{
				Event: &flow.Flow{
					IP: &flow.IP{
						Source: sourceIP,
					},
				},
			}
			enricher.Write(ev)
			time.Sleep(25 * time.Millisecond)
		}
	}()

	enricher.Run()

	wg.Add(1)
	go func() {
		defer wg.Done()
		count := 0
		reader := enricher.ExportReader()
		for {
			ev := reader.NextFollow(ctx)
			if ev == nil {
				break
			}
			fl := ev.Event.(*flow.Flow)
			receivedFlow := fl.GetSource()

			assert.NotNil(t, receivedFlow, "Expected flow")
			assert.Equal(t, "pod1", receivedFlow.GetPodName())
			assert.Equal(t, "ns1", receivedFlow.GetNamespace())

			count++
		}
		assert.Equal(t, expectedOutputCount, count, "Received event count mismatch")
	}()

	time.Sleep(3 * time.Second)
	cancel()
	wg.Wait()
}

func TestEnricherStandaloneWithEndpointAbsent(t *testing.T) {
	opts := log.GetDefaultLogOpts()
	opts.Level = "debug"
	if _, err := log.SetupZapLogger(opts); err != nil {
		t.Errorf("Error setting up logger: %s", err)
	}

	eventCount := 20
	expectedOutputCount := eventCount - 2

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	testCache := standalone.New()
	sourceIP := "9.9.9.9" // No endpoint present in cache

	// Create the enricher with standalone enabled
	enricher := newStandalone(ctx, testCache)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < eventCount; i++ {
			ev := &v1.Event{
				Event: &flow.Flow{
					IP: &flow.IP{
						Source: sourceIP,
					},
				},
			}
			enricher.Write(ev)
			time.Sleep(25 * time.Millisecond)
		}
	}()

	enricher.Run()

	wg.Add(1)
	go func() {
		defer wg.Done()
		count := 0
		reader := enricher.ExportReader()
		for {
			ev := reader.NextFollow(ctx)
			if ev == nil {
				break
			}
			fl := ev.Event.(*flow.Flow)
			receivedFlow := fl.GetSource()
			assert.Nil(t, receivedFlow)

			count++
		}
		assert.Equal(t, expectedOutputCount, count, "Received event count mismatch")
	}()

	time.Sleep(3 * time.Second)
	cancel()
	wg.Wait()
}
