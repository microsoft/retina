// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package apiserver

import "context"

//go:generate mockgen -source=types.go -destination=mocks/mock_types.go -package=mocks .
type IHostResolver interface {
	LookupHost(context context.Context, host string) ([]string, error)
}

// define cache as a set
type cache map[string]struct{}

func (c cache) deepcopy() cache {
	copy := make(cache)
	for k, v := range c {
		copy[k] = v
	}
	return copy
}
