// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package enricher

import (
	"sync"

	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
)

var (
	e           *Enricher
	once        sync.Once
	initialized bool
)

type Enricher struct{}

func New() *Enricher {
	once.Do(func() {
		e = &Enricher{}
		initialized = true
	})
	return e
}

func Instance() *Enricher {
	return e
}

func IsInitialized() bool {
	return initialized
}

func (e *Enricher) Run() {}

func (e *Enricher) Write(_ *v1.Event) {}

func (e *Enricher) ExportReader() RingReaderInterface { return nil }
