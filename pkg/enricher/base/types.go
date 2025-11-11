// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package base

import (
	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	"github.com/cilium/cilium/pkg/hubble/container"
)

//go:generate go run go.uber.org/mock/mockgen@v0.4.0 -destination=mock_enricherinterface.go -copyright_file=../lib/ignore_headers.txt -package=base github.com/microsoft/retina/pkg/enricher/base EnricherInterface

type EnricherInterface interface {
	Run()
	Write(ev *v1.Event)
	ExportReader() *container.RingReader
	Enrich(ev *v1.Event)
}
