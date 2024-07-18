package dns_notify_forwarding

import (
	"context"

	// "github.com/cilium/cilium/api/v1/flow"
	"github.com/cilium/cilium/pkg/hive/cell"
	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"

	// monitoragent "github.com/cilium/cilium/pkg/monitor/agent"
	// "github.com/cilium/cilium/pkg/proxy/accesslog"
	// plogger "github.com/cilium/cilium/pkg/proxy/logger"
	monitoragent "github.com/cilium/cilium/pkg/monitor/agent"
	"github.com/microsoft/retina/pkg/log"

	// "github.com/microsoft/retina/pkg/monitoragent"
	"go.uber.org/zap"
)

type DNSNotifyForwarding struct {
	l             *log.ZapLogger
	monitorAgent  monitoragent.Agent
	EventsChannel chan *v1.Event
	// cilium connection information
	// 40045 default port for cilium agent
	// 40046 for DNS proxy
}

func NewDNSNotifyForwarding(l *log.ZapLogger, monitorAgent monitoragent.Agent, eventsChan chan *v1.Event) *DNSNotifyForwarding {
	return &DNSNotifyForwarding{
		l:             l,
		monitorAgent:  monitorAgent,
		EventsChannel: eventsChan,
	}
}

// Connect to grpc client
// Or use monitor agent to send events 129 (see retina/pkg/monitoragent - needs updates)
func (d *DNSNotifyForwarding) Connect() error {
	// connect to cilium agent
	return nil
}

// Convert event to Log and record
func (d *DNSNotifyForwarding) ConvertAndRecord(event *v1.Event) error {
	// f := event.GetFlow()
	// flowType := accesslog.FlowType(f.L7.Type)
	// var addrInfo logger.AddressingInfo

	// if f.L7.Type == flow.L7FlowType_RESPONSE {
	// 	addrInfo.DstIPPort = epIPPort
	// 	addrInfo.DstIdentity = ep.GetIdentity()
	// 	addrInfo.SrcIPPort = serverAddr
	// 	addrInfo.SrcIdentity = serverID
	// } else {
	// 	flowType = accesslog.TypeRequest
	// 	addrInfo.SrcIPPort = epIPPort
	// 	addrInfo.SrcIdentity = ep.GetIdentity()
	// 	addrInfo.DstIPPort = serverAddr
	// 	addrInfo.DstIdentity = serverID
	// }
	// record := plogger.NewLogRecord(flowType, false,
	// 	func(lr *plogger.LogRecord) { lr.LogRecord.TransportProtocol = accesslog.TransportProtocol(protoID) },
	// 	plogger.LogTags.Verdict(f.GetVerdict(), f.Reason),
	// 	plogger.LogTags.Addressing(addrInfo),
	// 	plogger.LogTags.DNS(&accesslog.LogRecordDNS{
	// 		Query:             qname,
	// 		IPs:               responseIPs,
	// 		TTL:               TTL,
	// 		CNAMEs:            CNAMEs,
	// 		ObservationSource: stat.DataSource,
	// 		RCode:             rcode,
	// 		QTypes:            qTypes,
	// 		AnswerTypes:       recordTypes,
	// 	}),
	// )
	// record.Log()
	// monitoragent.SendEvent(event)
	return nil
}

// continuously listen for events from the channel and send to cilium agent
func (d *DNSNotifyForwarding) ListenAndForward(ctx context.Context) {
	for {
		select {
		case event := <-d.EventsChannel:
			// create mappings
			// send event to cilium agent
			// log event
			d.l.Info("DNSNOTIFY Received event", zap.String("event", event.GetFlow().String()))
			d.ConvertAndRecord(event)
		case <-ctx.Done():
			d.l.Info("stopping DNSNotifyForwarding")
			return
		}
	}
}

var dnsNotifyForwardCell = cell.Module(
	"DNSNotifyForwarding",
	"Forward DNS events to grpc connection",
	cell.Provide(func(l *log.ZapLogger, m monitoragent.Agent, e chan *v1.Event) *DNSNotifyForwarding {
		return NewDNSNotifyForwarding(l, m, e)
	}),
	cell.Invoke(
		func(d DNSNotifyForwarding, lifecycle cell.Lifecycle) {
			ctx, cancelCtx := context.WithCancel(context.Background())
			lifecycle.Append(cell.Hook{
				OnStart: func(cell.HookContext) error {
					d.l.Info("starting DNSNotifyForwarding")
					// setup connection
					// listen for events and forward
					go d.ListenAndForward(ctx)
					return nil
				},
				OnStop: func(cell.HookContext) error {
					cancelCtx()
					return nil
				},
			},
			)
		},
	),
)
