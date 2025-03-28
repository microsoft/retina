package parser

import (
	"errors"

	"github.com/cilium/cilium/api/v1/flow"
	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	observer "github.com/cilium/cilium/pkg/hubble/observer/types"
	ipc "github.com/cilium/cilium/pkg/ipcache"
	"github.com/cilium/cilium/pkg/k8s"
	"github.com/microsoft/retina/pkg/hubble/parser/layer34"
	"github.com/microsoft/retina/pkg/hubble/parser/seven"
	"github.com/sirupsen/logrus"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	errV1Event        = errors.New("failed to cast agent event to v1.Event")
	errEnrich         = errors.New("failed to enrich flow")
	errEmptyPayload   = errors.New("empty payload")
	errUnknownPayload = errors.New("unknown payload")
)

type Parser struct {
	l       logrus.FieldLogger
	ipcache *ipc.IPCache
	svc     k8s.ServiceCache

	l34 *layer34.Parser
	l7  *seven.Parser
}

func New(l *logrus.Entry, svc k8s.ServiceCache, c *ipc.IPCache) *Parser {
	return &Parser{
		l:       l,
		ipcache: c,
		svc:     svc,

		l34: layer34.New(l, svc, c),
		l7:  seven.New(l, svc, c),
	}
}

func (p *Parser) Decode(monitorEvent *observer.MonitorEvent) (*v1.Event, error) {
	switch monitorEvent.Payload.(type) { //nolint:gocritic
	case *observer.AgentEvent:
		payload := monitorEvent.Payload.(*observer.AgentEvent)
		ev, ok := payload.Message.(*v1.Event)
		if !ok {
			return nil, errV1Event
		}
		f := p._decode(ev)
		if f == nil {
			return nil, errEnrich
		}
		ev.Event = f
		ev.Timestamp = timestamppb.Now()
		return ev, nil
	case nil:
		return nil, errEmptyPayload
	default:
		return nil, errUnknownPayload
	}
}

func (p *Parser) _decode(event *v1.Event) *flow.Flow {
	if event == nil {
		return nil
	}

	// Enrich the event with the IP address of the source and destination.
	// This is used to enrich the event with the source and destination
	// node names.
	f, ok := event.Event.(*flow.Flow)
	if !ok {
		p.l.Warn("Failed to cast event to flow", zap.Any("event", event.Event))
		return nil
	}
	if f == nil {
		p.l.Warn("Failed to get flow from event", zap.Any("event", event))
		return nil
	}

	// Decode the flow based on its type.
	switch f.GetType() { //nolint:exhaustive // We only care about the known types.
	case flow.FlowType_L3_L4:
		f = p.l34.Decode(f)
	case flow.FlowType_L7:
		f = p.l7.Decode(f)
	default:
		p.l.Warn("Unknown flow type", zap.Any("flow", f))
	}

	p.l.Debug("Enriched flow", zap.Any("flow", f))
	return f
}
