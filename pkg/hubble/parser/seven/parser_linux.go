package seven

import (
	"fmt"
	"net/netip"
	"strings"

	"github.com/cilium/cilium/api/v1/flow"
	"github.com/cilium/cilium/pkg/ipcache"
	"github.com/google/gopacket/layers"
	"github.com/microsoft/retina/pkg/hubble/common"
	"github.com/sirupsen/logrus"
	"go.uber.org/zap"
)

type Parser struct {
	l  *logrus.Entry
	ep common.EpDecoder
}

func New(l *logrus.Entry, c *ipcache.IPCache) *Parser {
	return &Parser{
		l:  l.WithField("subsys", "seven"),
		ep: common.NewEpDecoder(c),
	}
}

func (p *Parser) Decode(f *flow.Flow) *flow.Flow {
	if f == nil {
		return nil
	}

	// Decode the flow's IP addresses to their respective endpoints.
	p.decodeIP(f)

	// Decode the flow's L7 protocol.
	l7 := f.GetL7()
	if l7 == nil {
		return f
	}

	record := l7.GetRecord()
	if record == nil {
		return f
	}

	switch record.(type) {
	case *flow.Layer7_Dns:
		return p.decodeDNS(f)
	case *flow.Layer7_Http:
		return p.decodeHTTP(f)
	}
	return f
}

func (p *Parser) decodeIP(f *flow.Flow) {
	if f == nil {
		return
	}

	// Decode the flow's source and destination IPs to their respective endpoints.
	if f.GetIP() == nil {
		p.l.Warn("Failed to get IP from flow", zap.Any("flow", f))
		return
	}
	sourceIP, err := netip.ParseAddr(f.GetIP().GetSource())
	if err != nil {
		p.l.Warn("Failed to parse source IP", zap.Error(err))
		return
	}
	destIP, err := netip.ParseAddr(f.GetIP().GetDestination())
	if err != nil {
		p.l.Warn("Failed to parse destination IP", zap.Error(err))
		return
	}

	f.Source = p.ep.Decode(sourceIP)
	f.Destination = p.ep.Decode(destIP)
}

func (p *Parser) decodeDNS(f *flow.Flow) *flow.Flow {
	l7 := f.GetL7()
	if l7 == nil {
		return f
	}

	dns := l7.GetDns()
	if dns != nil {
		//nolint:staticcheck // TODO(timraymond): no good migration path documented
		f.Summary = dnsSummary(dns, l7.GetType())
	}

	f.Verdict = flow.Verdict_FORWARDED

	return f
}

func (p *Parser) decodeHTTP(f *flow.Flow) *flow.Flow {
	l7 := f.GetL7()
	if l7 == nil {
		return f
	}

	// TODO need to implemented
	// noop for timebeing

	f.Verdict = flow.Verdict_FORWARDED
	return f
}

func dnsSummary(dns *flow.DNS, flowtype flow.L7FlowType) string {
	if len(dns.GetQtypes()) == 0 {
		return ""
	}
	qTypeStr := strings.Join(dns.GetQtypes(), ",")

	switch flowtype { //nolint:exhaustive // the other two types are "sample", and "unknown" which we can ignore
	case flow.L7FlowType_REQUEST:
		return fmt.Sprintf("DNS Query %s %s", dns.GetQuery(), qTypeStr)
	case flow.L7FlowType_RESPONSE:
		rcode := layers.DNSResponseCode(dns.GetRcode())

		var answer string
		if rcode != layers.DNSResponseCodeNoErr {
			answer = fmt.Sprintf("RCode: %s", rcode)
		} else {
			parts := make([]string, 0)

			if len(dns.GetIps()) > 0 {
				parts = append(parts, fmt.Sprintf("%q", strings.Join(dns.GetIps(), ",")))
			}

			if len(dns.GetCnames()) > 0 {
				parts = append(parts, fmt.Sprintf("CNAMEs: %q", strings.Join(dns.GetCnames(), ",")))
			}

			answer = strings.Join(parts, " ")
		}

		sourceType := "Query"

		return fmt.Sprintf("DNS Answer %s (%s %s %s)", answer, sourceType, dns.GetQuery(), qTypeStr)
	}

	return ""
}
