// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package enricher

import (
	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	"github.com/cilium/cilium/pkg/hubble/container"
)

//go:generate mockgen -destination=mock_enricherinterface.go  -copyright_file=../lib/ignore_headers.txt -package=enricher github.com/microsoft/retina/pkg/enricher EnricherInterface

type EnricherInterface interface {
	Run()
	Write(ev *v1.Event)
	ExportReader() *container.RingReader
}
