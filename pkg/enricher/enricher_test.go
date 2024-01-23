// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package enricher

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/microsoft/retina/pkg/common"
	"github.com/microsoft/retina/pkg/controllers/cache"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/pubsub"
	"github.com/cilium/cilium/api/v1/flow"
	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestNewEnricher(t *testing.T) {
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

	addEndpoints := common.NewRetinaEndpoint("pod1", "ns1", nil)
	addEndpoints.SetLabels(map[string]string{
		"app": "app1",
	})

	addEndpoints.SetOwnerRefs([]*common.OwnerReference{
		{
			Kind: "Pod",
			Name: "pod1-deployment",
		},
	})

	addEndpoints.SetIPs(&common.IPAddresses{
		IPv4: net.IPv4(1, 1, 1, 1),
	})
	err := c.UpdateRetinaEndpoint(addEndpoints)
	assert.NoError(t, err)

	e := New(ctx, c)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		for i := 0; i < evCount; i++ {
			addEvent(e)
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
			count++
		}
		assert.Equal(t, expectedOutputCount, count, "two")
		wg.Done()
	}()

	time.Sleep(3 * time.Second)
	cancel()
	wg.Wait()
}

func addEvent(e *Enricher) {
	l := log.Logger().Named("addev")
	ev := &v1.Event{
		Timestamp: timestamppb.Now(),
		Event: &flow.Flow{
			IP: &flow.IP{
				IpVersion:   1,
				Source:      "1.1.1.1",
				Destination: "2.2.2.2",
			},
		},
	}

	l.Info("Adding event", zap.Any("event", ev))
	time.Sleep(100 * time.Millisecond)
	e.Write(ev)
}
