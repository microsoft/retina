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
	"github.com/stretchr/testify/require"
)

const testIP = "10.0.0.0"

func TestPublishEvent(t *testing.T) {
	if _, err := log.SetupZapLogger(log.GetDefaultLogOpts()); err != nil {
		t.Errorf("Error setting up logger: %s", err)
	}
	MaxStandaloneCacheEventSize = 1

	tests := []struct {
		name        string
		fillChannel bool
		event       StandaloneEvent
		expectedErr error
	}{
		{
			name:        "Event published successfully",
			fillChannel: false,
			event:       StandaloneEvent{IP: testIP, Action: AddEvent},
			expectedErr: nil,
		},
		{
			name:        "Event channel is full",
			fillChannel: true,
			event:       StandaloneEvent{IP: testIP, Action: AddEvent},
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
				require.Error(t, err)
				require.Equal(t, tt.expectedErr, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRemoveStaleEntries(t *testing.T) {
	if _, err := log.SetupZapLogger(log.GetDefaultLogOpts()); err != nil {
		t.Errorf("Error setting up logger: %s", err)
	}
	testCache := cache.NewStandaloneCache(1 * time.Millisecond)
	e := NewStandaloneEnricher(context.Background(), testCache, &config.Config{})
	e.Run()
	defer e.Stop()

	podInfo := cache.PodInfo{Name: "retina-pod", Namespace: "retina-namespace", LastUpdate: time.Now()}

	testCache.Update(testIP, &podInfo)
	time.Sleep(10 * time.Millisecond)
	e.RemoveStaleEntries()

	require.Eventually(t, func() bool {
		return testCache.GetPod(testIP) == nil
	}, 100*time.Millisecond, 10*time.Millisecond, "Expected pod info should be nil after TTL expired")
}

func TestRun(t *testing.T) {
	if _, err := log.SetupZapLogger(log.GetDefaultLogOpts()); err != nil {
		t.Errorf("Error setting up logger: %s", err)
	}
	existingIP := "192.0.0.5"
	nonExistingIP := testIP

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
			setupCache:      func(_ *cache.StandaloneCache) {},
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
			setupCache:      func(_ *cache.StandaloneCache) {},
			expectedPodInfo: nil,
		},
		{
			name:            "No update for unknown event",
			event:           StandaloneEvent{IP: existingIP, Action: Action("unknown")},
			setupCache:      func(_ *cache.StandaloneCache) {},
			expectedPodInfo: nil,
		},
		{
			name:            "No update when event channel is closed",
			event:           StandaloneEvent{IP: existingIP, Action: AddEvent},
			setupCache:      func(_ *cache.StandaloneCache) {},
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
				err := e.PublishEvent(tt.event.IP, tt.event.Action)
				require.NoError(t, err)
			}

			time.Sleep(25 * time.Millisecond)

			podInfo := e.GetPodInfo(tt.event.IP)
			if tt.expectedPodInfo != nil {
				require.NotNil(t, podInfo)
				require.Equal(t, tt.expectedPodInfo.Name, podInfo.Name)
				require.Equal(t, tt.expectedPodInfo.Namespace, podInfo.Namespace)
			} else {
				require.Nil(t, podInfo)
			}
		})
	}
}
