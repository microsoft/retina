// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package enricher

import (
	"context"

	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	"github.com/cilium/cilium/pkg/hubble/container"
	"github.com/microsoft/retina/pkg/log"
)

//go:generate go run go.uber.org/mock/mockgen@v0.4.0 -destination=mock_enricherinterface.go  -copyright_file=../lib/ignore_headers.txt -package=enricher github.com/microsoft/retina/pkg/enricher EnricherInterface

type EnricherInterface interface {
	Run()
	Write(ev *v1.Event)
	ExportReader() *container.RingReader
}

type GenericEnricher struct {
	ctx context.Context
	l   *log.ZapLogger
}
