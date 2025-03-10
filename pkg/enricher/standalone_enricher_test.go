// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package enricher

import (
	"context"
	"testing"

	"github.com/microsoft/retina/pkg/controllers/cache"
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

	go enricher.Run(context.Background())

	// Event add for pod that doesn't exist in statefile
	enricher.ProcessEvent(invalidIp, cache.EventAdd)
	podInfo := testCache.GetPod(invalidIp)
	assert.Nil(t, podInfo)

	// Event add for pod that exists in statefile
	enricher.ProcessEvent(ip, cache.EventAdd)

	podInfo = testCache.GetPod(ip)
	assert.NotNil(t, podInfo)
	assert.Equal(t, name, podInfo.Name)
	assert.Equal(t, namespace, podInfo.Namespace)

	// Event delete for pod that doesn't exist in cache
	enricher.ProcessEvent(invalidIp, cache.EventDelete)
	podInfo = testCache.GetPod(invalidIp)
	assert.Nil(t, podInfo)

	// Test deleting a pod that exists in the cache
	enricher.ProcessEvent(ip, cache.EventDelete)
	podInfo = testCache.GetPod(ip)
	assert.Nil(t, podInfo) // cache should be empty now

	// Test unknown event action
	unknownAction := cache.Action("unknown")
	enricher.ProcessEvent("9.9.9.9", unknownAction)
	assert.Nil(t, testCache.GetPod("9.9.9.9")) // nothing should happen

	enricher.Stop()
}
