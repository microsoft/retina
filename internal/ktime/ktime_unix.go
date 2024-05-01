//go:build unix

package ktime

import (
	"time"

	"golang.org/x/sys/unix"
)

// calculateMonotonicOffset tries to determine the offset of the kernel's
// monotonic clock from UTC so that measurements from eBPF using the
// monotonic clock timestamp may be adjusted to wall-time.
//
// These instructions do not execute instantaneously so it will always be
// impossible to sample both clocks at exactly the same time.
// This means that for any single process there will be constant error in
// the accuracy of this measurement despite the nanosecond-level precision
// of the individual clocks.
func calculateMonotonicOffset() time.Duration {
	mono := &unix.Timespec{}
	now := time.Now()
	_ = unix.ClockGettime(unix.CLOCK_BOOTTIME, mono)
	return time.Duration(now.UnixNano() - unix.TimespecToNsec(*mono))
}
