// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package metrics

import (
	"slices"
	"testing"
	"testing/synctest"
	"time"

	api "github.com/microsoft/retina/crd/api/v1alpha1"
	"github.com/microsoft/retina/pkg/log"
)

func TestBaseMetricObject(t *testing.T) {
	l, err := log.SetupZapLogger(log.GetDefaultLogOpts())
	if err != nil {
		t.Fatalf("failed to set up logger: %v", err)
	}

	tests := []struct {
		name         string
		ttl          time.Duration
		trackMetrics bool
	}{
		{
			name:         "test base metric object zero ttl",
			ttl:          0,
			trackMetrics: false,
		},
		{
			name:         "test base metric object negative ttl",
			ttl:          -time.Millisecond,
			trackMetrics: false,
		},
		{
			name:         "test base metric object positive ttl",
			ttl:          time.Millisecond,
			trackMetrics: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The bubble gives the test a fake clock, so the expiration
			// ticker fires deterministically. It also fails the test if the
			// expiration goroutine is still alive when the bubble ends.
			synctest.Test(t, func(t *testing.T) {
				var expireCalled []string
				b := newBaseMetricsObject(
					&api.MetricsContextOptions{
						MetricName: "test_metric",
					},
					l,
					localContext,
					func(lbs []string) bool {
						expireCalled = lbs
						return true
					},
					tt.ttl,
				)
				t.Cleanup(b.clean)

				testLabels := []string{"test"}
				b.updated(testLabels)

				metrics := len(b.trackedMetricLabels())
				if tt.trackMetrics {
					if metrics != 1 {
						t.Errorf("expected 1 tracked metric label, got %d", metrics)
					}
				} else {
					if metrics != 0 {
						t.Errorf("expected 0 tracked metric labels, got %d", metrics)
					}
				}

				// If we have a positive TTL, we should see the expire function get called after the TTL has passed
				if tt.ttl > 0 {
					// Advance the fake clock to the first tick, then wait for
					// the expiration goroutine to block again.
					time.Sleep(tt.ttl)
					synctest.Wait()
					if !slices.Equal(expireCalled, testLabels) {
						t.Errorf("expected expire to be called with %v, got %v", testLabels, expireCalled)
					}
					metrics = len(b.trackedMetricLabels())
					if metrics != 0 {
						t.Errorf("expected 0 tracked metric labels after expiration, got %d", metrics)
					}
				} else if len(expireCalled) != 0 {
					t.Errorf("expected expire to not be called, but got %v", expireCalled)
				}
			})
		})
	}
}
