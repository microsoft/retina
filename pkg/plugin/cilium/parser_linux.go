package cilium

import (
	"errors"
	"time"

	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	observerTypes "github.com/cilium/cilium/pkg/hubble/observer/types"
	hp "github.com/cilium/cilium/pkg/hubble/parser"
	parserErrors "github.com/cilium/cilium/pkg/hubble/parser/errors"
	hptestutils "github.com/cilium/cilium/pkg/hubble/testutils"
	monitorAPI "github.com/cilium/cilium/pkg/monitor/api"
	"github.com/cilium/cilium/pkg/monitor/payload"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"go.uber.org/zap"
)

const ErrNotImplemented = "Error, not implemented"

var ErrEmptyData = errors.New("empty data")

func (p *parser) Init() {
	parser, err := hp.New(logrus.WithField("cilium", "parser"),
		// We use noOp getters here since we will use our own custom parser in hubble
		&hptestutils.NoopEndpointGetter,
		&hptestutils.NoopIdentityGetter,
		&hptestutils.NoopDNSGetter,
		&hptestutils.NoopIPGetter,
		&hptestutils.NoopServiceGetter,
		&hptestutils.NoopLinkGetter,
		&hptestutils.NoopPodMetadataGetter,
	)
	if err != nil {
		p.l.Fatal("Failed to create parser", zap.Error(err))
	}
	p.hparser = parser
}

// Reconstruct monitorEvents and then decode flow events to our hubble instance.
// Additionally, We require the corresponding parser in pkg/hubble/parser for
// hubble metrics to be enriched and updated properly
// Agent Events
//   - MessageTypeAccessLog:		accesslog.LogRecord
//   - MessageTypeAgent:			api.AgentNotify
//
// Perf Events
//   - MessageTypeDrop:				monitor.DropNotify
//   - MessageTypeDebug:			monitor.DebugMsg
//   - MessageTypeCapture:			monitor.DebugCapture
//   - MessageTypeTrace:			monitor.TraceNotify
//   - MessageTypePolicyVerdict:	monitor.PolicyVerdictNotify
func (p *parser) Decode(pl *payload.Payload) (*v1.Event, error) {
	switch pl.Type {
	case payload.EventSample:
		data := pl.Data
		if len(data) == 0 {
			return nil, ErrEmptyData
		}

		var eventType uint8 = data[0]

		monEvent := &observerTypes.MonitorEvent{
			Timestamp: time.Now(),
			UUID:      uuid.New(),
		}
		switch eventType {
		case monitorAPI.MessageTypeAccessLog, monitorAPI.MessageTypeAgent:
			// AgentEvents correlate to cilium agent events
			// This also includes access logs, which are Log Records
			// Log Records can be DNS traces for CNP related pods
			// AccessLogs can also reflect kafka related metrics
			monEvent.Payload = &observerTypes.AgentEvent{}
			return nil, errors.New(ErrNotImplemented) //nolint:goerr113 //no specific handling expected
		// MessageTypeTraceSock and MessageTypeDebug are also perf events but have their own dedicated decoders in cilium.
		case monitorAPI.MessageTypeDrop, monitorAPI.MessageTypeTrace, monitorAPI.MessageTypePolicyVerdict, monitorAPI.MessageTypeCapture:
			perfEvent := &observerTypes.PerfEvent{}
			perfEvent.Data = data
			perfEvent.CPU = pl.CPU
			monEvent.Payload = perfEvent
			event, err := p.hparser.Decode(monEvent)
			if err != nil {
				return nil, err //nolint:wrapcheck // dont wrap error since it would not provide more context
			}
			return event, nil
		default:
			return nil, parserErrors.ErrUnknownEventType
		}
	case payload.RecordLost:
		p.l.Warn("Record lost for cilium event", zap.Uint64("lost", pl.Lost))
		return nil, errors.New("Record lost for cilium event") //nolint:goerr113 //no specific handling expected
	default:
		p.l.Warn("Unknown event type", zap.Int("type", pl.Type))
		return nil, parserErrors.ErrUnknownEventType
	}
}
