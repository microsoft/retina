//go:build !unix

package ktime

import "time"

func calculateMonotonicOffset() time.Duration {
	return 0 * time.Nanosecond
}
