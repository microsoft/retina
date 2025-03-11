// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package enricher

import (
	"context"
	"testing"
	"time"

	"github.com/microsoft/retina/pkg/controllers/cache"
	"github.com/microsoft/retina/pkg/enricher/statefile"
	"github.com/microsoft/retina/pkg/log"
	"github.com/stretchr/testify/assert"
)

const (
	ip        = "192.0.0.5"
	invalidIp = "10.0.0.0"
	name      = "retina2-pod"
	namespace = "retina2-namespace"
)

func TestStandaloneEnricher(t *testing.T) {
	if _, err := log.SetupZapLogger(log.GetDefaultLogOpts()); err != nil {
		t.Fatalf("Failed to setup logger: %v", err)
	}
	testCache := cache.NewStandaloneCache()
	enricher := NewStandaloneEnricher(context.Background(), testCache)

	go enricher.Run()

	// Enrich pod not in statefile
	err := enricher.PublishEvent(invalidIp)
	assert.NoError(t, err)

	podInfo := enricher.GetPodInfo(invalidIp)
	assert.Nil(t, podInfo)

	// Enrich pod in statefile
	err = enricher.PublishEvent(ip)
	assert.NoError(t, err)

	assert.Eventually(t, func() bool {
		podInfo = enricher.GetPodInfo(ip)
		return podInfo != nil
	}, 100*time.Millisecond, 25*time.Millisecond, "Pod should be available in cache")
	assert.Equal(t, name, podInfo.Name)
	assert.Equal(t, namespace, podInfo.Namespace)

	// Delete pod not in statefile
	statefile.State_file_location = "/home/beegii/src/retina/pkg/enricher/statefile/mock_statefile_updated.json"
	err = enricher.PublishEvent(ip)
	assert.NoError(t, err)

	assert.Eventually(t, func() bool {
		podInfo = enricher.GetPodInfo(ip)
		return podInfo == nil
	}, 100*time.Millisecond, 25*time.Millisecond, "Pod should be deleted from cache")
	assert.Nil(t, podInfo)

	enricher.Stop()
}

func TestPublishEvent(t *testing.T) {
	if _, err := log.SetupZapLogger(log.GetDefaultLogOpts()); err != nil {
		t.Fatalf("Failed to setup logger: %v", err)
	}

	MaxStandaloneCacheEventSize = 1
	testCache := cache.NewStandaloneCache()
	enricher := NewStandaloneEnricher(context.Background(), testCache)

	go enricher.Run()

	err1 := enricher.PublishEvent(ip)
	err2 := enricher.PublishEvent(invalidIp)
	assert.NoError(t, err1)
	assert.Equal(t, err2, ErrEventChannelFull)

	enricher.Stop()
}
