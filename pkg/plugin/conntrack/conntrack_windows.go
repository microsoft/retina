// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build windows
// +build windows

package conntrack

import "context"

type Conntrack struct{}

// Implement no-op functions for Windows.
func Init() (*Conntrack, error) {
	return nil, nil
}

func (ct *Conntrack) Close() {}

func (ct *Conntrack) Run(_ context.Context) error {
	return nil
}
