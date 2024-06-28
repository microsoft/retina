package conntrack

import (
	"context"
	"net/netip"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/cilium/cilium/api/v1/flow"
	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	"github.com/cilium/workerpool"
	"github.com/mdlayher/netlink"
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/metrics"
	"github.com/microsoft/retina/pkg/plugin/api"
	"github.com/microsoft/retina/pkg/utils"
	"github.com/pkg/errors"
	"github.com/ti-mo/conntrack"
	"github.com/ti-mo/netfilter"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	podSubnet     netip.Prefix
	serviceSubnet netip.Prefix
	nodeIP        string
)

type Monitor struct {
	cfg             *kcfg.Config
	l               *log.ZapLogger
	ct              *conntrack.Conn
	externalChannel chan *v1.Event
	eventCh         chan conntrack.Event
	activeConn      sync.Map     // max entries should be the maximum number of flows conntrack was set to track
	tcpState        *tcpStateMap // max entries should be the maximum number of TCP states
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
	m.tcpState = newTCPStateMap()
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
	// Dump the current conntrack table and initialize the activeConn map
	// with the current connections
	entries, err := m.ct.Dump(&conntrack.DumpOptions{})
	if err != nil {
		m.l.Error("Failed to dump conntrack table", zap.Error(err))
		return errors.Wrapf(err, "failed to dump conntrack table")
	}
	for i := 0; i < len(entries); i++ {
		fl := entries[i]
		addr := fl.TupleOrig.IP.DestinationAddress
		if podSubnet.Contains(addr) || serviceSubnet.Contains(addr) || addr.String() == nodeIP {
			continue
		}
		switch fl.TupleOrig.Proto.Protocol {
		case TCP:
			m.activeConn.Store(fl.ID, 1)
			port := strconv.FormatUint(uint64(fl.TupleOrig.Proto.DestinationPort), 10)
			metrics.TCPConnectionRemoteGauge.WithLabelValues(addr.String(), port).Inc()
			// Increment the TCP state in the tcpState map
			state := fl.ProtoInfo.TCP.State
			m.tcpState.inc(state)
		case UDP:
			m.activeConn.Store(fl.ID, 1)
			metrics.UDPConnectionStats.WithLabelValues(utils.Active).Inc()
		default:
		}
	}
	// Make a buffered channel to receive event updates on.
	evCh := make(chan conntrack.Event, 1024) // nolint:gomnd // 1024 is a reasonable buffer size.
	m.eventCh = evCh

	// Listen for all Conntrack events with 4 decoder goroutines.
	// All errors caught in the decoders are passed on channel errCh.
	errCh, err := m.ct.Listen(evCh, 8, netfilter.GroupsCT) // nolint:gomnd // 4 is a reasonable number of decoders.
	if err != nil {
		m.l.Error("Failed to listen for conntrack events", zap.Error(err))
		return errors.Wrapf(err, "failed to listen for conntrack events")
	}

	// Set the option to avoid receiving ENOBUFS errors.
	// Note that if the buffer is full, the kernel will drop packets.
	// This might lead to userspace state misalignment with the kernel state but
	// allows for higher throughput.
	err = m.ct.SetOption(netlink.NoENOBUFS, true)
	if err != nil {
		m.l.Error("Failed to set option to avoid ENOBUFS errors", zap.Error(err))
		return errors.Wrap(err, "failed to set option to avoid ENOBUFS errors")
	}

	// Start a goroutine to log errors from the errCh
	go func() {
		err := <-errCh
		m.l.Error("Conntrack event listener error", zap.Error(err))
	}()

	// Start a workerpool to process conntrack events
	wp := workerpool.NewWithContext(ctx, 8) //nolint:gomnd // 4 is a reasonable number of workers.
	m.l.Info("Starting workerpool to process conntrack events...")
	if err := wp.Submit("process-conntrack-events", m.processConntrackEvents); err != nil {
		return errors.Wrap(err, "failed to submit process-conntrack-events to workerpool")
	}

	// Start a goroutine to update the TCP state metrics at interval specified in the config
	ticker := time.NewTicker(m.cfg.MetricsInterval)
	defer ticker.Stop()
	go func() {
		for {
			select {
			case <-ctx.Done():
				m.l.Debug("Context is done, tcp state monitor will stop running")
				return
			case <-ticker.C:
				m.tcpState.updateMetrics()
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

func (m *Monitor) SetupChannel(ch chan *v1.Event) error {
	m.l.Info("Setting up channel for conntrack monitor plugin...")
	m.externalChannel = ch
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
			if m.externalChannel != nil {
				var l4 *flow.Layer4
				switch ev.Flow.TupleOrig.Proto.Protocol {
				case TCP:
					l4 = &flow.Layer4{
						Protocol: &flow.Layer4_TCP{
							TCP: &flow.TCP{
								SourcePort:      uint32(ev.Flow.TupleOrig.Proto.SourcePort),
								DestinationPort: uint32(ev.Flow.TupleOrig.Proto.DestinationPort),
							},
						},
					}
				case UDP:
					l4 = &flow.Layer4{
						Protocol: &flow.Layer4_UDP{
							UDP: &flow.UDP{
								SourcePort:      uint32(ev.Flow.TupleOrig.Proto.SourcePort),
								DestinationPort: uint32(ev.Flow.TupleOrig.Proto.DestinationPort),
							},
						},
					}
				}
				fl := &flow.Flow{
					Time: timestamppb.New(ev.Flow.Timestamp.Start),
					Uuid: strconv.FormatUint(uint64(ev.Flow.ID), 10),
					Type: flow.FlowType_L3_L4,
					IP: &flow.IP{
						Source:      ev.Flow.TupleOrig.IP.SourceAddress.String(),
						Destination: ev.Flow.TupleOrig.IP.DestinationAddress.String(),
						// We only support IPv4 for now.
						IpVersion: flow.IPVersion_IPv4,
					},
					L4:               l4,
					TrafficDirection: flow.TrafficDirection_EGRESS,
				}
				hubbleEv := &v1.Event{
					Timestamp: fl.GetTime(),
					Event:     fl,
				}
				select {
				case m.externalChannel <- hubbleEv:
				default:
					// Channel is full, drop the event.
					// We shouldn't slow down the reader.
					metrics.LostEventsCounter.WithLabelValues(utils.ExternalChannel, string(Name)).Inc()
				}
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
			// Add the new TCP connection to the activeConn map
			_, loaded := m.activeConn.LoadOrStore(ev.Flow.ID, 1)
			if loaded {
				m.l.Debug("New TCP connection already exists in activeConn map", zap.String("flow", ev.Flow.TupleOrig.String()))
			} else {
				metrics.TCPConnectionRemoteGauge.WithLabelValues(dstAddr.String(), dstPort).Inc()
			}
			// Increment the TCP state in the tcpState map
			state := ev.Flow.ProtoInfo.TCP.State
			m.tcpState.inc(state)
		}
	case UDP:
		m.l.Debug("Received new UDP conntrack event", zap.String("event", ev.String()))
		dstAddr := ev.Flow.TupleOrig.IP.DestinationAddress
		if !podSubnet.Contains(dstAddr) && !serviceSubnet.Contains(dstAddr) && dstAddr.String() != nodeIP {
			_, loaded := m.activeConn.LoadOrStore(ev.Flow.ID, 1)
			if loaded {
				m.l.Warn("New UDP connection already exists in activeConn map", zap.String("flow", ev.Flow.TupleOrig.String()))
			} else {
				metrics.UDPConnectionStats.WithLabelValues(utils.Active).Inc()
			}
		}
	default:
	}
}

func (m *Monitor) processDestroyConntrackEvents(ev conntrack.Event) {
	switch ev.Flow.TupleOrig.Proto.Protocol {
	case TCP:
		m.l.Debug("Received destroy TCP conntrack event", zap.String("event", ev.String()))
		_, loaded := m.activeConn.LoadAndDelete(ev.Flow.ID)
		if !loaded {
			m.l.Debug("Destroyed TCP connection not found in activeConn map", zap.String("flow", ev.Flow.TupleOrig.String()))
		} else {
			addr := ev.Flow.TupleOrig.IP.DestinationAddress
			port := strconv.FormatUint(uint64(ev.Flow.TupleOrig.Proto.DestinationPort), 10)
			metrics.TCPConnectionRemoteGauge.WithLabelValues(addr.String(), port).Dec()
		}
		// Increment the TCP state in the tcpState map
		state := ev.Flow.ProtoInfo.TCP.State
		m.tcpState.inc(state)
	case UDP:
		m.l.Debug("Received destroy UDP conntrack event", zap.String("event", ev.String()))
		_, loaded := m.activeConn.LoadAndDelete(ev.Flow.ID)
		if !loaded {
			m.l.Debug("Destroyed UDP connection not found in activeConn map", zap.String("flow", ev.Flow.TupleOrig.String()))
		} else {
			metrics.UDPConnectionStats.WithLabelValues(utils.Active).Dec()
		}
	default:
	}
}

func (m *Monitor) processUpdateConntrackEvents(ev conntrack.Event) {
	m.l.Debug("Received update conntrack event", zap.String("event", ev.String()))
	switch ev.Flow.TupleOrig.Proto.Protocol {
	case TCP:
		if ev.Flow.ProtoInfo.TCP == nil {
			m.l.Debug("Received update TCP conntrack event without TCP info", zap.String("event", ev.String()))
			return
		}
		// Increment the TCP state in the tcpState map
		state := ev.Flow.ProtoInfo.TCP.State
		m.tcpState.inc(state)
	case UDP:
		m.l.Debug("Received update UDP conntrack event", zap.String("event", ev.String()))
	default:
	}

}

func TCPState(tcpstate uint8) string {
	// Define the mapping of uint8 values to TCP states
	// from https://github.com/torvalds/linux/blob/v6.2/include/net/tcp_states.h#L12-L27
	tcpStates := map[uint8]string{
		1:  "ESTABLISHED",
		2:  "SYN_SENT",
		3:  "SYN_RECV",
		4:  "FIN_WAIT1",
		5:  "FIN_WAIT2",
		6:  "TIME_WAIT",
		7:  "CLOSE",
		8:  "CLOSE_WAIT",
		9:  "LAST_ACK",
		10: "LISTEN",
		11: "CLOSING",
		12: "NEW_SYN_RECV",
	}

	// Check if the uint8 value corresponds to a valid TCP state
	if state, ok := tcpStates[tcpstate]; ok {
		return state
	}
	return "UNKNOWN"
}
