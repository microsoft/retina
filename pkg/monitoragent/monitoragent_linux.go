package monitoragent

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"

	"github.com/cilium/cilium/api/v1/models"
	observerTypes "github.com/cilium/cilium/pkg/hubble/observer/types"
	"github.com/cilium/cilium/pkg/lock"
	"github.com/cilium/cilium/pkg/monitor/agent/consumer"
	"github.com/cilium/cilium/pkg/monitor/agent/listener"
	"github.com/cilium/cilium/pkg/monitor/api"
	"github.com/cilium/cilium/pkg/monitor/payload"
	"github.com/sirupsen/logrus"
)

var (
	errMonitorAgentNotSetup = fmt.Errorf("monitor agent is not set up")
	errUnexpectedEvent      = errors.New("unexpected event type for MessageTypeAgent")
)

// isCtxDone is a utility function that returns true when the context's Done()
// channel is closed. It is intended to simplify goroutines that need to check
// this multiple times in their loop.
func isCtxDone(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

type monitorAgent struct {
	lock.Mutex
	models.MonitorStatus

	ctx              context.Context
	perfReaderCancel context.CancelFunc

	// listeners are external cilium monitor clients which receive raw
	// gob-encoded payloads
	listeners map[listener.MonitorListener]struct{}
	// consumers are internal clients which receive decoded messages
	consumers map[consumer.MonitorConsumer]struct{}
}

func (a *monitorAgent) AttachToEventsMap(int) error {
	return nil
}

func (a *monitorAgent) SendEvent(typ int, event interface{}) error {
	if a == nil {
		return errMonitorAgentNotSetup
	}

	// Two types of clients are currently supported: consumers and listeners.
	// The former ones expect decoded messages, so the notification does not
	// require any additional marshalling operation before sending an event.
	// Instead, the latter expect gob-encoded payloads, and the whole marshalling
	// process may be quite expensive.
	// While we want to avoid marshalling events if there are no active
	// listeners, there's no need to check for active consumers ahead of time.

	log.Info("SendEvent called")
	// perfEvents are bpf datapath events (reserved for 0-128), where 0 = unspecified
	if typ > 0 && typ <= 128 {
		log.Info("SendEvent called with PERF event")
		event, ok := event.(observerTypes.PerfEvent)
		if !ok {
			return fmt.Errorf("unexpected event type for perf event")
		}
		a.sendPerfEvent(event)
	} else {
		log.Info("SendEvent called with AGENT event")
		a.notifyAgentEvent(typ, event)
	}

	// do not marshal notifications if there are no active listeners
	if !a.hasListeners() {
		return nil
	}

	// marshal notifications into JSON format for legacy listeners
	if typ == api.MessageTypeAgent {
		msg, ok := event.(api.AgentNotifyMessage)
		if !ok {
			return errUnexpectedEvent
		}
		var err error
		event, err = msg.ToJSON()
		if err != nil {
			return fmt.Errorf("unable to JSON encode agent notification: %w", err)
		}
	}

	var buf bytes.Buffer
	if err := buf.WriteByte(byte(typ)); err != nil {
		return fmt.Errorf("unable to initialize buffer: %w", err)
	}
	if err := gob.NewEncoder(&buf).Encode(event); err != nil {
		return fmt.Errorf("unable to gob encode: %w", err)
	}

	p := payload.Payload{Data: buf.Bytes(), CPU: 0, Lost: 0, Type: payload.EventSample}
	a.sendToListeners(&p)

	return nil
}

func (a *monitorAgent) sendPerfEvent(event observerTypes.PerfEvent) {
	a.Lock()
	defer a.Unlock()
	a.notifyPerfEventLocked(event.Data, event.CPU)
}

// notifyPerfEventLocked notifies all consumers about a perf event.
// The caller must hold the monitor lock.
func (a *monitorAgent) notifyPerfEventLocked(data []byte, cpu int) {
	for mc := range a.consumers {
		mc.NotifyPerfEvent(data, cpu)
	}
}

func (a *monitorAgent) RegisterNewListener(newListener listener.MonitorListener) {
	if a == nil || newListener == nil {
		return
	}

	a.Lock()
	defer a.Unlock()

	if isCtxDone(a.ctx) {
		log.Debug("RegisterNewListener called on stopped monitor")
		newListener.Close()
		return
	}

	version := newListener.Version()
	switch newListener.Version() { //nolint:exhaustive // the only other case is unsupported which is covered by default
	case listener.Version1_2:
		a.listeners[newListener] = struct{}{}
	default:
		newListener.Close()
		log.WithField("version", version).Error("Closing listener from unsupported monitor client version")
	}

	log.WithFields(logrus.Fields{
		"count.listener": len(a.listeners),
		"version":        version,
	}).Debug("New listener connected")
}

func (a *monitorAgent) RemoveListener(ml listener.MonitorListener) {
	if a == nil || ml == nil {
		return
	}

	a.Lock()
	defer a.Unlock()

	// Remove the listener and close it.
	delete(a.listeners, ml)
	log.WithFields(logrus.Fields{
		"count.listener": len(a.listeners),
		"version":        ml.Version(),
	}).Debug("Removed listener")
	ml.Close()
}

func (a *monitorAgent) RegisterNewConsumer(newConsumer consumer.MonitorConsumer) {
	if a == nil || newConsumer == nil {
		return
	}

	if isCtxDone(a.ctx) {
		log.Debug("RegisterNewConsumer called on stopped monitor")
		return
	}

	a.Lock()
	defer a.Unlock()

	a.consumers[newConsumer] = struct{}{}
}

func (a *monitorAgent) RemoveConsumer(mc consumer.MonitorConsumer) {
	if a == nil || mc == nil {
		return
	}

	a.Lock()
	defer a.Unlock()

	delete(a.consumers, mc)
	if !a.hasSubscribersLocked() {
		a.perfReaderCancel()
	}
}

func (a *monitorAgent) State() *models.MonitorStatus {
	return nil
}

// hasSubscribersLocked returns true if there are listeners or consumers
// subscribed to the agent right now.
// Note: it is critical to hold the lock for this operation.
func (a *monitorAgent) hasSubscribersLocked() bool {
	return len(a.listeners)+len(a.consumers) != 0
}

// hasListeners returns true if there are listeners subscribed to the
// agent right now.
func (a *monitorAgent) hasListeners() bool {
	a.Lock()
	defer a.Unlock()
	return len(a.listeners) != 0
}

// sendToListeners enqueues the payload to all listeners.
func (a *monitorAgent) sendToListeners(pl *payload.Payload) {
	a.Lock()
	defer a.Unlock()
	a.sendToListenersLocked(pl)
}

// sendToListenersLocked enqueues the payload to all listeners while holding the monitor lock.
func (a *monitorAgent) sendToListenersLocked(pl *payload.Payload) {
	for ml := range a.listeners {
		ml.Enqueue(pl)
	}
}

// notifyAgentEvent notifies all consumers about an agent event.
func (a *monitorAgent) notifyAgentEvent(typ int, message interface{}) {
	a.Lock()
	defer a.Unlock()
	for mc := range a.consumers {
		mc.NotifyAgentEvent(typ, message)
	}
}
