// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package utils

import (
	"errors"
	"testing"
	"time"
)

func TestBackoffWithJitterNeverShorterThanSchedule(t *testing.T) {
	for attempt := 0; attempt < 6; attempt++ {
		base := time.Duration(1<<attempt) * time.Second

		for i := 0; i < 500; i++ {
			got := backoffWithJitter(attempt)
			if got < base || got > 2*base {
				t.Fatalf("attempt %d: got %v, want within [%v, %v]", attempt, got, base, 2*base)
			}
		}
	}
}

func TestBackoffWithJitterVaries(t *testing.T) {
	seen := make(map[time.Duration]struct{})
	for i := 0; i < 500; i++ {
		seen[backoffWithJitter(3)] = struct{}{}
	}

	// A deterministic implementation returns a single value. The range at
	// attempt 3 is four seconds wide, so one distinct value across 500 draws
	// would not be chance.
	if len(seen) <= 1 {
		t.Fatalf("backoff must vary so agents do not retry in lockstep, got %v", seen)
	}
}

func TestRetryReturnsOnFirstSuccess(t *testing.T) {
	calls := 0
	err := Retry(func() error {
		calls++

		return nil
	}, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestRetryStopsAfterSuccess(t *testing.T) {
	calls := 0
	wantErr := errors.New("boom")
	err := Retry(func() error {
		calls++
		if calls < 2 {
			return wantErr
		}

		return nil
	}, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
}

func TestRetryReturnsLastErrorAfterExhaustion(t *testing.T) {
	wantErr := errors.New("boom")
	calls := 0
	// One attempt keeps the test fast: the first sleep is under a second.
	err := Retry(func() error {
		calls++

		return wantErr
	}, 1)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}
