// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package enricher

import (
	"context"
	"testing"

	"github.com/microsoft/retina/pkg/controllers/cache"
	"github.com/microsoft/retina/pkg/log"
	"github.com/stretchr/testify/mock"
)

type MockStandaloneCache struct {
	mock.Mock
}

func (m *MockStandaloneCache) WatchEvents() <-chan cache.Event {
	args := m.Called()
	return args.Get(0).(<-chan cache.Event)
}

func (m *MockStandaloneCache) AddPod(ip, name, namespace string) {
	m.Called(ip, name, namespace)
}

func (m *MockStandaloneCache) DeletePod(ip string) {
	m.Called(ip)
}

func MockGetPodInfo(ip, stateFileLocation string) (enricher.PodInfo, error) {
	return enricher.PodInfo{
		Name:      "test-pod",
		Namespace: "test-namespace",
	}, nil
}

func TestStandaloneEnricher(t *testing.T) {
	if _, err := log.SetupZapLogger(log.GetDefaultLogOpts()); err != nil {
		t.Fatalf("Failed to setup logger: %v", err)
	}
	mockCache := new(MockStandaloneCache)

	ctx := context.Background()
	enricher := NewStandaloneEnricher(ctx, mockCache)

	// override GetPodInfo function
	enricher.GetPodInfo = MockGetPodInfo

	mockCache.On("AddPod", "10.0.0.1", "test-pod", "test-namespace").Once()
	mockCache.On("DeletePod", "10.0.0.1").Once()
	mockCache.On("Unknown", mock.Anything, mock.Anything, mock.Anything).Maybe()

	eventsCh := make(chan cache.Event, 3)
	mockCache.On("WatchEvents").Return(eventsCh)

	// Start the enricher in a separate goroutine
	go enricher.Run(ctx)

	// Simulate sending EventAdd
	eventsCh <- cache.Event{
		Action:  cache.EventAdd,
		Ip:      "10.0.0.1",
		PodInfo: enricher.PodInfo{Name: "test-pod", Namespace: "test-namespace"},
	}

	// Simulate sending EventDelete
	eventsCh <- cache.Event{
		Action:  cache.EventDelete,
		Ip:      "10.0.0.1",
		PodInfo: enricher.PodInfo{Name: "pod2", Namespace: "namespace2"},
	}

	// Simulate sending an unknown event
	eventsCh <- cache.Event{
		Action:  "Unknown",
		Ip:      "10.0.0.1",
		PodInfo: nil,
	}

	mockCache.AssertExpectations(t)
	close(eventsCh)
}
