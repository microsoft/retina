// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package enricher

import (
	"context"

	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
)

//go:generate go run go.uber.org/mock/mockgen@v0.4.0 -destination=mock_enricherinterface.go  -copyright_file=../lib/ignore_headers.txt -package=enricher github.com/microsoft/retina/pkg/enricher EnricherInterface

type EnricherInterface interface {
	Run()
	Write(ev *v1.Event)
	ExportReader() RingReaderInterface
}

type RingReaderInterface interface {
	Previous() (*v1.Event, error)
	Next() (*v1.Event, error)
	NextFollow(ctx context.Context) *v1.Event
	Close() error
}
