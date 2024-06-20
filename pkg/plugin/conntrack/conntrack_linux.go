package conntrack

import (
	"context"
	"net/netip"
	"os"
	"strconv"
	"sync"
	"time"

	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	"github.com/cilium/workerpool"
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/metrics"
	"github.com/microsoft/retina/pkg/plugin/api"
	"github.com/microsoft/retina/pkg/utils"
	"github.com/pkg/errors"
	"github.com/ti-mo/conntrack"
	"github.com/ti-mo/netfilter"
	"go.uber.org/zap"
)

var (
	podSubnet     netip.Prefix
	serviceSubnet netip.Prefix
	nodeIP        string
)

type Monitor struct {
	cfg        *kcfg.Config
	l          *log.ZapLogger
	ct         *conntrack.Conn
	eventCh    chan conntrack.Event
	activeConn sync.Map // max entries should be the maximum number of flows conntrack was set to track
	tcpState   sync.Map // max entries should be the maximum number of TCP states
}

func New(cfg *kcfg.Config) api.Plugin {
	return &Monitor{
		cfg: cfg,
		l:   log.Logger().Named(string(Name)),
	}
}

func (m *Monitor) Name() string {
	return string(Name)
}

func (m *Monitor) Generate(_ context.Context) error {
	return nil
}

func (m *Monitor) Compile(_ context.Context) error {
	return nil
}

func (m *Monitor) Init() error {
	m.l.Info("Initializing conntrack monitor plugin...")
	m.activeConn = sync.Map{}
	m.tcpState = sync.Map{}
	// Parse PodCIDR and ServiceCIDR to get subnet
	var err error
	podSubnet, err = netip.ParsePrefix(PodCIDR)
	if err != nil {
		m.l.Error("Failed to parse PodCIDR", zap.Error(err))
		return errors.Wrapf(err, "failed to parse PodCIDR")
	}
	serviceSubnet, err = netip.ParsePrefix(ServiceCIDR)
	if err != nil {
		m.l.Error("Failed to parse ServiceCIDR", zap.Error(err))
		return errors.Wrapf(err, "failed to parse ServiceCIDR")
	}

	nodeIP = os.Getenv("NODE_IP")
	if nodeIP == "" {
		m.l.Error("NODE_IP environment variable is not set")
		return errors.New("NODE_IP environment variable is not set")
	}

	return nil
}

func (m *Monitor) Start(ctx context.Context) error {
	m.l.Info("Starting conntrack monitor plugin...")
	var err error
	m.ct, err = conntrack.Dial(nil)
	if err != nil {
		m.l.Error("Failed to dial conntrack", zap.Error(err))
		return errors.Wrapf(err, "failed to dial conntrack")
	}
	// Make a buffered channel to receive event updates on.
	evCh := make(chan conntrack.Event, 1024) // nolint:gomnd // 1024 is a reasonable buffer size.
	m.eventCh = evCh

	// Listen for all Conntrack and Conntrack-Expect events with 4 decoder goroutines.
	// All errors caught in the decoders are passed on channel errCh.
	errCh, err := m.ct.Listen(evCh, 4, netfilter.GroupsCT) // nolint:gomnd // 4 is a reasonable number of decoders.
	if err != nil {
		m.l.Error("Failed to listen for conntrack events", zap.Error(err))
		return errors.Wrapf(err, "failed to listen for conntrack events")
	}

	// Start a goroutine to log errors from the errCh
	go func() {
		err := <-errCh
		m.l.Error("Conntrack event listener error", zap.Error(err))
	}()

	// Start a workerpool to process conntrack events
	wp := workerpool.NewWithContext(ctx, 4) //nolint:gomnd // 4 is a reasonable number of workers.
	m.l.Info("Starting workerpool to process conntrack events...")
	if err := wp.Submit("process-conntrack-events", m.processConntrackEvents); err != nil {
		return errors.Wrap(err, "failed to submit process-conntrack-events to workerpool")
	}

	// Start a goroutine that, for every 30s, gauge the number of tcp states and then
	// reset the value in the map
	ticker := time.NewTicker(m.cfg.MetricsInterval)
	defer ticker.Stop()
	go func() {
		for {
			select {
			case <-ctx.Done():
				m.l.Debug("Context is done, tcp state monitor will stop running")
				return
			case <-ticker.C:
				m.tcpState.Range(func(key, value interface{}) bool {
					state := key.(uint8)
					count := value.(int)
					metrics.TCPStateGauge.WithLabelValues(getTCPState(state)).Set(float64(count))
					// Reset the value in the map
					m.tcpState.Store(state, 0)
					return true
				})
			}
		}
	}()

	// Block until the context is done
	<-ctx.Done()
	return nil
}

func (m *Monitor) Stop() error {
	m.l.Info("Stopping conntrack monitor plugin...")
	if m.ct != nil {
		m.ct.Close()
	}
	return nil
}

func (m *Monitor) SetupChannel(_ chan *v1.Event) error {
	m.l.Info("Setting up channel for conntrack monitor plugin...")
	return nil
}

func (m *Monitor) processConntrackEvents(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			m.l.Info("Context is done, conntrack event processor will stop running")
			return nil
		case ev := <-m.eventCh:
			switch ev.Type {
			case conntrack.EventNew:
				m.processNewConntrackEvents(ev)
			case conntrack.EventDestroy:
				m.processDestroyConntrackEvents(ev)
			case conntrack.EventUpdate:
				m.processUpdateConntrackEvents(ev)
			case conntrack.EventExpNew:
			case conntrack.EventExpDestroy:
			case conntrack.EventUnknown:
			default:
			}

		}
	}
}

func (m *Monitor) processNewConntrackEvents(ev conntrack.Event) {
	switch ev.Flow.TupleOrig.Proto.Protocol {
	case TCP:
		m.l.Debug("Received new TCP conntrack event", zap.String("event", ev.String()))
		// Check if the flow is to an external IP, not to pod or service CIDR or to the current node
		dstAddr := ev.Flow.TupleOrig.IP.DestinationAddress
		if !podSubnet.Contains(dstAddr) && !serviceSubnet.Contains(dstAddr) && dstAddr.String() != nodeIP {
			dstPort := strconv.FormatUint(uint64(ev.Flow.TupleOrig.Proto.DestinationPort), 10)
			val, loaded := m.activeConn.LoadOrStore(ev.Flow.TupleOrig, 1)
			if loaded {
				m.activeConn.Store(ev.Flow.TupleOrig, val.(int)+1)
			}
			// Increment the TCP state in the tcpState map
			state := ev.Flow.ProtoInfo.TCP.State
			val, loaded = m.tcpState.LoadOrStore(state, 1)
			if loaded {
				m.tcpState.Store(state, val.(int)+1)
			}
			metrics.TCPConnectionRemoteGauge.WithLabelValues(dstAddr.String(), dstPort).Inc()
		}
	case UDP:
		m.l.Debug("Received new UDP conntrack event", zap.String("event", ev.String()))
		dstAddr := ev.Flow.TupleOrig.IP.DestinationAddress
		if !podSubnet.Contains(dstAddr) && !serviceSubnet.Contains(dstAddr) && dstAddr.String() != nodeIP {
			val, loaded := m.activeConn.LoadOrStore(ev.Flow.TupleOrig, 1)
			if loaded {
				m.activeConn.Store(ev.Flow.TupleOrig, val.(int)+1)
			}
			metrics.UDPConnectionStats.WithLabelValues(utils.Active).Inc()
		}
	default:
	}
}

func (m *Monitor) processDestroyConntrackEvents(ev conntrack.Event) {
	switch ev.Flow.TupleOrig.Proto.Protocol {
	case TCP:
		m.l.Debug("Received destroy TCP conntrack event", zap.String("event", ev.String()))
		addr := ev.Flow.TupleOrig.IP.DestinationAddress
		port := strconv.FormatUint(uint64(ev.Flow.TupleOrig.Proto.DestinationPort), 10)
		val, loaded := m.activeConn.LoadOrStore(ev.Flow.TupleOrig, 0)
		if loaded {
			switch val.(int) {
			case 1: // If there is only one connection, remove it from the map and decrement the gauge
				m.activeConn.Delete(ev.Flow.TupleOrig)
				metrics.TCPConnectionRemoteGauge.WithLabelValues(addr.String(), port).Dec()
			case 0: // If there are no connections, remove it from the map but do not decrement the gauge to prevent negative values
				m.activeConn.Delete(ev.Flow.TupleOrig)
			default: // Otherwise, decrement the value in the map and the gauge
				m.activeConn.Store(ev.Flow.TupleOrig, val.(int)-1)
				metrics.TCPConnectionRemoteGauge.WithLabelValues(addr.String(), port).Dec()
			}
		} else {
			m.l.Debug("Destroyed TCP connection not found in activeConn map", zap.String("flow", ev.Flow.TupleOrig.String()))
		}
		// Increment the TCP state in the tcpState map
		state := ev.Flow.ProtoInfo.TCP.State
		val, loaded = m.tcpState.LoadOrStore(state, 1)
		if loaded {
			m.tcpState.Store(state, val.(int)+1)
		}
	case UDP:
		m.l.Debug("Received destroy UDP conntrack event", zap.String("event", ev.String()))
		val, loaded := m.activeConn.LoadOrStore(ev.Flow.TupleOrig, 0)
		if loaded {
			switch val.(int) {
			case 1: // If there is only one connection, remove it from the map and decrement the gauge
				m.activeConn.Delete(ev.Flow.TupleOrig)
				metrics.UDPConnectionStats.WithLabelValues(utils.Active).Dec()
			case 0: // If there are no connections, remove it from the map but do not decrement the gauge to prevent negative values
				m.activeConn.Delete(ev.Flow.TupleOrig)
			default: // Otherwise, decrement the value in the map and the gauge
				m.activeConn.Store(ev.Flow.TupleOrig, val.(int)-1)
				metrics.UDPConnectionStats.WithLabelValues(utils.Active).Dec()
			}
		} else {
			m.l.Debug("Destroyed UDP connection not found in activeConn map", zap.String("flow", ev.Flow.TupleOrig.String()))
		}
	default:
	}
}

func (m *Monitor) processUpdateConntrackEvents(ev conntrack.Event) {
	m.l.Debug("Received update conntrack event", zap.String("event", ev.String()))
	switch ev.Flow.TupleOrig.Proto.Protocol {
	case TCP:
		// Increment the TCP state in the tcpState map
		state := ev.Flow.ProtoInfo.TCP.State
		val, loaded := m.tcpState.LoadOrStore(state, 1)
		if loaded {
			m.tcpState.Store(state, val.(int)+1)
		}
	case UDP:
		m.l.Info("Received update UDP conntrack event", zap.String("event", ev.String()))
	default:
	}

}

func getTCPState(uint8Value uint8) string {
	// Define the mapping of uint8 values to TCP states
	tcpStates := map[uint8]string{
		0:  "CLOSED",
		1:  "LISTEN",
		2:  "SYN_SENT",
		3:  "SYN_RECEIVED",
		4:  "ESTABLISHED",
		5:  "FIN_WAIT_1",
		6:  "FIN_WAIT_2",
		7:  "CLOSE_WAIT",
		8:  "CLOSING",
		9:  "LAST_ACK",
		10: "TIME_WAIT",
	}

	// Check if the uint8 value corresponds to a valid TCP state
	if state, ok := tcpStates[uint8Value]; ok {
		return state
	}
	return "UNKNOWN"
}
