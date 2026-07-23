// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package metrics

import (
	"runtime"
	"slices"
	"testing"
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
			before := runtime.NumGoroutine()
			expireCalled := new([]string)
			b := newBaseMetricsObject(
				&api.MetricsContextOptions{
					MetricName: "test_metric",
				},
				l,
				localContext,
				func(lbs []string) bool {
					*expireCalled = lbs
					return true
				},
				tt.ttl,
			)

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
				time.Sleep(tt.ttl + time.Millisecond*100)
				if !slices.Equal(*expireCalled, testLabels) {
					t.Errorf("expected expire to be called with %v, got %v", testLabels, *expireCalled)
				}
				metrics = len(b.trackedMetricLabels())
				if metrics != 0 {
					t.Errorf("expected 0 tracked metric labels after expiration, got %d", metrics)
				}
			} else if len(*expireCalled) != 0 {
				t.Errorf("expected expire to not be called, but got %v", *expireCalled)
			}

			b.clean()
			if b.expireFn != nil {
				<-b.ctx.Done()
			}

			// Wait for any goroutines to exit after clean is called
			if tt.trackMetrics {
				time.Sleep(tt.ttl + time.Millisecond*100)
			}

			after := runtime.NumGoroutine()
			if after != before {
				t.Errorf("expected number of goroutines to be the same as before, expected %d, got %d", before, after)
			}
		})
	}
}
