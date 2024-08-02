// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package cilium

import (
	"net"

	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	"github.com/cilium/proxy/pkg/lock"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
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
	l      *log.ZapLogger
	packet *packet
}

// re-usable packet to avoid reallocating gopacket datastructures
type packet struct {
	lock.Mutex
	decLayer *gopacket.DecodingLayerParser
	Layers   []gopacket.LayerType
	layers.Ethernet
	layers.IPv4
	layers.IPv6
	// layers.ICMPv4
	// layers.ICMPv6
	layers.TCP
	layers.UDP
	// layers.SCTP
}
