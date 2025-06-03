package layer34

import (
	"fmt"
	"net/netip"

	"github.com/cilium/cilium/api/v1/flow"
	ipc "github.com/cilium/cilium/pkg/ipcache"
	"github.com/cilium/cilium/pkg/k8s"

	"github.com/microsoft/retina/pkg/hubble/common"
	"github.com/microsoft/retina/pkg/utils"
	"github.com/sirupsen/logrus"
	"go.uber.org/zap"
)

type Parser struct {
	l   *logrus.Entry
	svd common.SvcDecoder
	epd common.EpDecoder
}

func New(l *logrus.Entry, svc k8s.ServiceCache, c *ipc.IPCache) *Parser {
	p := &Parser{
		l:   l.WithField("subsys", "layer34"),
		svd: common.NewSvcDecoder(svc),
		epd: common.NewEpDecoder(c),
	}
	// Log the localHostIP for debugging purposes.
	return p
}

// Decode enriches the flow with metadata from the IP cache and service cache.
func (p *Parser) Decode(f *flow.Flow) *flow.Flow {
	if f == nil {
		return nil
	}
	if f.GetIP() == nil {
		p.l.Warn("Failed to get IP from flow", zap.Any("flow", f))
		return f
	}
	sourceIP, err := netip.ParseAddr(f.GetIP().GetSource())
	if err != nil {
		p.l.Warn("Failed to parse source IP", zap.Error(err))
		return f
	}
	destIP, err := netip.ParseAddr(f.GetIP().GetDestination())
	if err != nil {
		p.l.Warn("Failed to parse destination IP", zap.Error(err))
		return f
	}

	// Decode the flow's source and destination IPs to their respective endpoints.
	f.Source = p.epd.Decode(sourceIP)
	f.Destination = p.epd.Decode(destIP)
	f.SourceService = p.svd.Decode(sourceIP)
	f.DestinationService = p.svd.Decode(destIP)

	// Add L34 Summary to flow.
	p.decodeSummary(f)

	p.decodeTrafficDirection(f)

	return f
}

func (p *Parser) decodeSummary(f *flow.Flow) {
	if f.GetVerdict() == flow.Verdict_DROPPED {
		// Setting subtype to DROPPED for huuble cli.
		if f.GetEventType() != nil {
			f.GetEventType().SubType = int32(f.GetDropReasonDesc())
			//nolint:lll // long line is long
			f.Summary = fmt.Sprintf("Drop Reason: %s\nNote: This reason is most accurate. Prefer over others while using Hubble CLI.", utils.DropReasonDescription(f)) // nolint:staticcheck // We need summary for now.
		}
		return

	}

	// Add Summary based off of L4 protocol.
	// Needed for huuble cli.
	if f.GetL4() != nil && f.GetL4().GetProtocol() != nil {
		switch f.GetL4().GetProtocol().(type) {
		case *flow.Layer4_TCP:
			tcpFlags := f.GetL4().GetTCP().GetFlags()
			if tcpFlags != nil {
				f.Summary = "TCP Flags: " + tcpFlags.String() // nolint:staticcheck // We need summary for now.
			}
		case *flow.Layer4_UDP:
			f.Summary = "UDP" // nolint:staticcheck // We need summary for now.
		}
	}
}

// decodeTrafficDirection decodes the traffic direction of the flow.
// It is only required for DROPPED verdicts because dropreason bpf program
// cannot determine the traffic direction. We determine using the source endpoint's
// node IP.
// Note: If the source and destination are on the same node, then the traffic is outbound.
func (p *Parser) decodeTrafficDirection(f *flow.Flow) {
	// Only required for DROPPED verdicts.
	if f.GetVerdict() != flow.Verdict_DROPPED {
		return
	}

	// If the source EP's node is the same as the current node, then the traffic is outbound.
	if p.epd.IsEndpointOnLocalHost(f.GetIP().GetSource()) {
		f.TrafficDirection = flow.TrafficDirection_EGRESS
		return
	}

	// Default to ingress.
	f.TrafficDirection = flow.TrafficDirection_INGRESS
}
