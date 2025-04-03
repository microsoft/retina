// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package enricher

import (
	"context"
	"testing"
	"time"

	"github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/controllers/cache"
	"github.com/microsoft/retina/pkg/enricher/statefile"
	"github.com/microsoft/retina/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	ip        = "192.0.0.5"
	invalidIP = "10.0.0.0"
	name      = "retina2-pod"
	namespace = "retina2-namespace"
	testfile  = "statefile/mock_statefile.json"
)

func TestPublishEvent(t *testing.T) {
	if _, err := log.SetupZapLogger(log.GetDefaultLogOpts()); err != nil {
		t.Fatalf("Failed to setup logger: %v", err)
	}

	statefile.StateFileLocation = testfile
	MaxStandaloneCacheEventSize = 1
	testCache := cache.NewStandaloneCache()
	config := &config.Config{
		EnableCrictl: false,
	}

	enricher := NewStandaloneEnricher(context.Background(), testCache, config)

	go enricher.Run()
	defer enricher.Stop()

	err1 := enricher.PublishEvent(ip)
	err2 := enricher.PublishEvent(invalidIP)
	require.NoError(t, err1)
	assert.Equal(t, err2, ErrEventChannelFull)
}

func TestStandaloneEnricher(t *testing.T) {
	if _, err := log.SetupZapLogger(log.GetDefaultLogOpts()); err != nil {
		t.Fatalf("Failed to setup logger: %v", err)
	}
	// Set the state file location to a mock file for testing
	statefile.StateFileLocation = testfile

	testCache := cache.NewStandaloneCache()
	config := &config.Config{
		EnableCrictl: false,
	}

	enricher := NewStandaloneEnricher(context.Background(), testCache, config)

	go enricher.Run()
	defer enricher.Stop()

	// Enrich pod not in statefile
	err := enricher.PublishEvent(invalidIP)
	require.NoError(t, err)

	podInfo := enricher.GetPodInfo(invalidIP)
	assert.Nil(t, podInfo)

	// Enrich pod in statefile
	err = enricher.PublishEvent(ip)
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		podInfo = enricher.GetPodInfo(ip)
		return podInfo != nil
	}, 100*time.Millisecond, 25*time.Millisecond, "Pod should be available in cache")
	assert.Equal(t, name, podInfo.Name)
	assert.Equal(t, namespace, podInfo.Namespace)

	// Delete pod not in statefile
	statefile.StateFileLocation = "statefile/mock_statefile_updated.json"
	err = enricher.PublishEvent(ip)
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		podInfo = enricher.GetPodInfo(ip)
		return podInfo == nil
	}, 100*time.Millisecond, 25*time.Millisecond, "Pod should be deleted from cache")
	assert.Nil(t, podInfo)
}
