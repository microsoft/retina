// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

// package apiretry provides the retry logic for API calls.
package apiretry

import (
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"
)

// Do will retry the do func only when the error is transient.
func Do(do func() error) error {
	backOffPeriod := retry.DefaultBackoff
	backOffPeriod.Cap = time.Second * 1

	return retry.OnError(backOffPeriod, func(err error) bool {
		return retriable(err)
	}, do)
}

func retriable(err error) bool {
	if apierrors.IsTimeout(err) ||
		apierrors.IsServerTimeout(err) ||
		apierrors.IsTooManyRequests(err) {
		return true
	}
	return false
}
