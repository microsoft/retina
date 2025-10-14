// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package ciliumeventobserver

import (
	"net"
	"time"

	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	hp "github.com/cilium/cilium/pkg/hubble/parser"
	"github.com/cilium/cilium/pkg/monitor/payload"
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/enricher/base"
	"github.com/microsoft/retina/pkg/log"
)

const name = "ciliumeventobserver"

type ciliumeventobserver struct {
	cfg             *kcfg.Config
	l               *log.ZapLogger
	enricher        base.EnricherInterface
	externalChannel chan *v1.Event
	payloadEvents   chan *payload.Payload
	connection      net.Conn
	p               *parser
	maxAttempts     int
	retryDelay      time.Duration
	sockPath        string
	d               dialer
}

type dialer interface {
	Dial(network, address string) (net.Conn, error)
}

type DefaultDialer struct{}

type parser struct {
	l       *log.ZapLogger
	hparser *hp.Parser
}
