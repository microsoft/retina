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

type Parser struct {
	l        logrus.FieldLogger
	ipcache  *ipc.IPCache
	svccache *k8s.ServiceCache

	l34 *layer34.Parser
	l7  *seven.Parser
}

func New(l *logrus.Entry, c *ipc.IPCache, sc *k8s.ServiceCache) *Parser {
	return &Parser{
		l:        l,
		ipcache:  c,
		svccache: sc,

		l34: layer34.New(l, c, sc),
		l7:  seven.New(l, c, sc),
	}
}

func (p *Parser) Decode(monitorEvent *observer.MonitorEvent) (*v1.Event, error) {
	switch monitorEvent.Payload.(type) { //nolint:typeSwitchVar
	case *observer.AgentEvent:
		payload := monitorEvent.Payload.(*observer.AgentEvent)
		ev, ok := payload.Message.(*v1.Event)
		if !ok {
			return nil, errors.New("failed to cast agent event to v1.Event")
		}
		f := p._decode(ev)
		if f == nil {
			return nil, errors.New("failed to enrich flow")
		}
		ev.Event = f
		ev.Timestamp = timestamppb.Now()
		return ev, nil
	case nil:
		return nil, errors.New("empty payload")
	default:
		return nil, errors.New("unknown payload")
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
	switch f.Type {
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
