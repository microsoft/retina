// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package dns

import (
	"sync"

	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	"github.com/cilium/ebpf/perf"
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/log"
)

const name = "dns"

type dns struct {
	cfg             *kcfg.Config
	l               *log.ZapLogger
	enricher        enricher.EnricherInterface
	externalChannel chan *v1.Event
	objs            *dnsObjects
	reader          *perf.Reader
	sock            int
	isRunning       bool
	recordsChannel  chan perf.Record
	wg              sync.WaitGroup
}
