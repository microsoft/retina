// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package cilium

import (
	"net"

	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	hp "github.com/cilium/cilium/pkg/hubble/parser"
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/plugin/api"
)

const (
	Name api.PluginName = "cilium"
)

type cilium struct {
	cfg             *kcfg.Config
	l               *log.ZapLogger
	enricher        enricher.EnricherInterface
	externalChannel chan *v1.Event
	connection      net.Conn
	p               *parser
}

type parser struct {
	l       *log.ZapLogger
	hparser *hp.Parser
}
