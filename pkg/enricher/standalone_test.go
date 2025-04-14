// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package enricher

import (
	"context"
	"testing"
	"time"

	"github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/controllers/cache"
	sf "github.com/microsoft/retina/pkg/enricher/statefile"
	"github.com/microsoft/retina/pkg/log"
	"github.com/stretchr/testify/assert"
)

func TestPublishEvent(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	MaxStandaloneCacheEventSize = 1
	ip := "10.0.0.0"

	tests := []struct {
		name        string
		fillChannel bool
		event       StandaloneEvent
		expectedErr error
	}{
		{
			name:        "Event published successfully",
			fillChannel: false,
			event:       StandaloneEvent{IP: ip, Action: AddEvent},
			expectedErr: nil,
		},
		{
			name:        "Event channel is full",
			fillChannel: true,
			event:       StandaloneEvent{IP: ip, Action: AddEvent},
			expectedErr: ErrEventChannelFull,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.fillChannel {
				MaxStandaloneCacheEventSize = 0
			}

			testCache := cache.NewStandaloneCache(10 * time.Second)
			e := NewStandaloneEnricher(context.Background(), testCache, &config.Config{})

			err := e.PublishEvent(tt.event.IP, tt.event.Action)
			if tt.expectedErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedErr, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRemoveStaleEntries(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())

	testCache := cache.NewStandaloneCache(1 * time.Millisecond)
	e := NewStandaloneEnricher(context.Background(), testCache, &config.Config{})
	e.Run()
	defer e.Stop()

	ip := "10.0.0.0"
	podInfo := cache.PodInfo{Name: "retina-pod", Namespace: "retina-namespace", LastUpdate: time.Now()}

	testCache.Update(ip, &podInfo)
	time.Sleep(10 * time.Millisecond)
	e.RemoveStaleEntries()

	assert.Eventually(t, func() bool {
		return testCache.GetPod(ip) == nil
	}, 100*time.Millisecond, 10*time.Millisecond, "Expected pod info should be nil after TTL expired")
}

func TestRun(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	existingIP := "192.0.0.5"
	nonExistingIP := "10.0.0.0"
	name := "retina-pod"
	namespace := "retina-namespace"
	sf.StateFileLocation = "statefile/mock_statefile.json"

	tests := []struct {
		name            string
		event           StandaloneEvent
		setupCache      func(c *cache.StandaloneCache)
		expectedPodInfo *cache.PodInfo
		shutdown        bool
	}{
		{
			name:            "Successful cache update",
			event:           StandaloneEvent{IP: existingIP, Action: AddEvent},
			setupCache:      func(c *cache.StandaloneCache) {},
			expectedPodInfo: &cache.PodInfo{Name: name, Namespace: namespace},
		},
		{
			name:  "Successful cache deletion",
			event: StandaloneEvent{IP: existingIP, Action: DeleteEvent},
			setupCache: func(c *cache.StandaloneCache) {
				podInfo := cache.PodInfo{Name: name, Namespace: namespace}
				c.Update(existingIP, &podInfo)
			},
			expectedPodInfo: nil,
		},
		{
			name:            "No update when pod info is empty",
			event:           StandaloneEvent{IP: nonExistingIP, Action: AddEvent},
			setupCache:      func(c *cache.StandaloneCache) {},
			expectedPodInfo: nil,
		},
		{
			name:            "No update for unknown event",
			event:           StandaloneEvent{IP: existingIP, Action: Action("unknown")},
			setupCache:      func(c *cache.StandaloneCache) {},
			expectedPodInfo: nil,
		},
		{
			name:            "No update when event channel is closed",
			event:           StandaloneEvent{IP: existingIP, Action: AddEvent},
			setupCache:      func(c *cache.StandaloneCache) {},
			shutdown:        true,
			expectedPodInfo: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testCache := cache.NewStandaloneCache(1 * time.Second)
			e := NewStandaloneEnricher(context.Background(), testCache, &config.Config{EnableCrictl: false})

			if tt.setupCache != nil {
				tt.setupCache(testCache)
			}

			e.Run()

			if tt.shutdown {
				e.Stop()
			} else {
				e.PublishEvent(tt.event.IP, tt.event.Action)
			}

			time.Sleep(25 * time.Millisecond)

			podInfo := e.GetPodInfo(tt.event.IP)
			if tt.expectedPodInfo != nil {
				assert.NotNil(t, podInfo)
				assert.Equal(t, tt.expectedPodInfo.Name, podInfo.Name)
				assert.Equal(t, tt.expectedPodInfo.Namespace, podInfo.Namespace)
			} else {
				assert.Nil(t, podInfo)
			}
		})
	}
}
