// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package tcpretrans

import (
	"errors"
	"sync"

	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	"github.com/cilium/ebpf/perf"
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/log"
)

const name = "tcpretrans"

type tcpretrans struct {
	cfg             *kcfg.Config
	l               *log.ZapLogger
	enricher        enricher.EnricherInterface
	externalChannel chan *v1.Event
	objs            *tcpretransObjects
	reader          *perf.Reader
	hooks           []interface{ Close() error }
	isRunning       bool
	recordsChannel  chan perf.Record
	wg              sync.WaitGroup
}

var errEnricherNotInitialized = errors.New("enricher not initialized")
