package ebpfwindows

import (
    "context"
    "errors"
    "net"
    "time"
    "unsafe"

    "github.com/cilium/cilium/api/v1/flow"
    v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
    hp "github.com/cilium/cilium/pkg/hubble/parser"
    kcfg "github.com/microsoft/retina/pkg/config"
    "github.com/microsoft/retina/pkg/enricher"
    "github.com/microsoft/retina/pkg/log"
    "github.com/microsoft/retina/pkg/metrics"
    "github.com/microsoft/retina/pkg/plugin/registry"
    "github.com/microsoft/retina/pkg/utils"
    "github.com/sirupsen/logrus"
    "go.uber.org/zap"
)

const (
    // name of the ebpfwindows plugin
    name string = "windowseBPF"
    // name of the metrics
    packetsReceived        string = "win_packets_recv_count"
    packetsSent            string = "win_packets_sent_count"
    bytesSent              string = "win_bytes_sent_count"
    bytesReceived          string = "win_bytes_recv_count"
    droppedPacketsIncoming string = "win_packets_recv_drop_count"
    droppedPacketsOutgoing string = "win_packets_sent_drop_count"
    // metrics direction
    ingressLabel = "ingress"
    egressLabel  = "egress"
)

var (
    ErrInvalidEventData = errors.New("The Cilium Event Data is invalid")
    ErrNilEnricher      = errors.New("enricher is nil")
)

// Plugin is the ebpfwindows plugin
type Plugin struct {
    l               *log.ZapLogger
    cfg             *kcfg.Config
    enricher        *enricher.Enricher
    externalChannel chan *v1.Event
    parser          *hp.Parser
}

func init() {
    registry.Add(name, New)
}

func New(cfg *kcfg.Config) registry.Plugin {
    return &Plugin{
        l:   log.Logger().Named(name),
        cfg: cfg,
    }
}

// Init is a no-op for the ebpfwindows plugin
func (p *Plugin) Init() error {

    parser, err := hp.New(logrus.WithField("cilium", "parser"),
        nil,
        nil,
        nil,
        nil,
        nil,
        nil,
        nil,
    )

    if err != nil {
        p.l.Fatal("Failed to create parser", zap.Error(err))
        return err
    }

    p.parser = parser
    return nil
}

// Name returns the name of the ebpfwindows plugin
func (p *Plugin) Name() string {
    return name
}

// Start the plugin by starting a periodic timer.
func (p *Plugin) Start(ctx context.Context) error {

    p.l.Info("Start ebpfWindows plugin...")
    p.enricher = enricher.Instance()

    if p.enricher == nil {
        return ErrNilEnricher
    }

    p.pullCiliumMetricsAndEvents(ctx)
    return nil
}

// metricsMapIterateCallback is the callback function that is called for each key-value pair in the metrics map.
func (p *Plugin) metricsMapIterateCallback(key *MetricsKey, value *MetricsValues) {
    p.l.Info("MetricsMapIterateCallback")
    p.l.Info("Key", zap.String("Key", key.String()))
    p.l.Info("Value", zap.String("Value", value.String()))

    if key.IsDrop() {
        if key.IsEgress() {
            metrics.DropPacketsGauge.WithLabelValues(egressLabel).Set(float64(value.Count()))
        } else if key.IsIngress() {
            metrics.DropPacketsGauge.WithLabelValues(ingressLabel).Set(float64(value.Count()))
        }

    } else {

        if key.IsEgress() {
            metrics.ForwardBytesGauge.WithLabelValues(egressLabel).Set(float64(value.Bytes()))
            p.l.Debug("emitting bytes sent count metric", zap.Uint64(bytesSent, value.Bytes()))
            metrics.WindowsGauge.WithLabelValues(packetsSent).Set(float64(value.Count()))
            p.l.Debug("emitting packets sent count metric", zap.Uint64(packetsSent, value.Count()))
        } else if key.IsIngress() {
            metrics.ForwardPacketsGauge.WithLabelValues(ingressLabel).Set(float64(value.Count()))
            p.l.Debug("emitting packets received count metric", zap.Uint64(packetsReceived, value.Count()))
            metrics.ForwardBytesGauge.WithLabelValues(ingressLabel).Set(float64(value.Bytes()))
            p.l.Debug("emitting bytes received count metric", zap.Uint64(bytesReceived, value.Bytes()))
        }
    }
}

// eventsMapCallback is the callback function that is called for each value  in the events map.
func (p *Plugin) eventsMapCallback(data unsafe.Pointer, size uint64) int {
    p.l.Info("EventsMapCallback")
    p.l.Info("Size", zap.Uint64("Size", size))
    err := p.handleTraceEvent(data, size)

    if err != nil {
        p.l.Error("Error handling trace event", zap.Error(err))
        return -1
    }

    return 0
}

// pullCiliumeBPFMetrics is the function that is called periodically by the timer.
func (p *Plugin) pullCiliumMetricsAndEvents(ctx context.Context) {

    eventsMap := NewEventsMap()
    metricsMap := NewMetricsMap()

    err := eventsMap.RegisterForCallback(p.eventsMapCallback)

    if err != nil {
        p.l.Error("Error registering for events map callback", zap.Error(err))
        return
    }

    ticker := time.NewTicker(p.cfg.MetricsInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            err := metricsMap.IterateWithCallback(p.metricsMapIterateCallback)
            if err != nil {
                p.l.Error("Error iterating metrics map", zap.Error(err))
            }
        case <-ctx.Done():
            p.l.Error("ebpfwindows plugin canceling", zap.Error(ctx.Err()))
            eventsMap.UnregisterForCallback()
            return
        }
    }
}

// SetupChannel saves the external channel to which the plugin will send events.
func (p *Plugin) SetupChannel(ch chan *v1.Event) error {
    p.externalChannel = ch
    return nil
}

// Stop the plugin by cancelling the periodic timer.
func (p *Plugin) Stop() error {

    p.l.Info("Stop ebpfWindows plugin...")
    return nil
}

// Compile is a no-op for the ebpfwindows plugin
func (p *Plugin) Compile(context.Context) error {
    return nil
}

// Generate is a no-op for the ebpfwindows plugin
func (p *Plugin) Generate(context.Context) error {
    return nil
}

func (p *Plugin) handleDropNotify(dropNotify *DropNotify) {
    p.l.Info("DropNotify", zap.String("DropNotify", dropNotify.String()))
}

func (p *Plugin) handleTraceNotify(traceNotify *TraceNotify) {
    p.l.Info("TraceNotify", zap.String("TraceNotify", traceNotify.String()))
}

func (p *Plugin) handleTraceSockNotify(traceSockNotify *TraceSockNotify) {
    p.l.Info("TraceSockNotify", zap.String("TraceSockNotify", traceSockNotify.String()))
}

func (p *Plugin) handleTraceEvent(data unsafe.Pointer, size uint64) error {

    if uintptr(size) < unsafe.Sizeof(uint8(0)) {
        return ErrInvalidEventData
    }

    eventType := *(*uint8)(data)

    switch eventType {
    case CiliumNotifyDrop:

        if uintptr(size) < unsafe.Sizeof(DropNotify{}) {
            p.l.Error("Invalid DropNotify data size", zap.Uint64("size", size))
            return ErrInvalidEventData
        }

        dropNotify := (*DropNotify)(data)
        p.handleDropNotify(dropNotify)

    case CiliumNotifyTrace:

        if uintptr(size) < unsafe.Sizeof(TraceNotify{}) {
            p.l.Error("Invalid TraceNotify data size", zap.Uint64("size", size))
            return ErrInvalidEventData
        }

        traceNotify := (*TraceNotify)(data)
        p.handleTraceNotify(traceNotify)

    case CiliumNotifyTraceSock:
        if uintptr(size) < unsafe.Sizeof(TraceSockNotify{}) {
            p.l.Error("Invalid TraceSockNotify data size", zap.Uint64("size", size))
            return ErrInvalidEventData
        }

        traceSockNotify := (*TraceSockNotify)(data)
        p.handleTraceSockNotify(traceSockNotify)

    default:
        p.l.Error("Unsupported event type", zap.Uint8("eventType", eventType))
    }

    t1 := time.Now().UnixNano()

    // Hardcoded values for flow object. These values will be replaced by the actual values from the event.
    fl := utils.ToFlow(
        p.l,
        t1,
        net.ParseIP("192.168.0.1").To4(), // Src IP
        net.ParseIP("192.168.0.2").To4(), // Dst IP
        80,                               // Src Port
        1024,                             // Dst Port
        6,                                // Protocol
        2,
        flow.Verdict_DROPPED,
    )

    if fl == nil {
        p.l.Warn("Could not convert event to flow", zap.Any("handleTraceEvent", data))
        return ErrInvalidEventData
    }

    ev := &v1.Event{
        Event:     fl,
        Timestamp: fl.GetTime(),
    }

    if p.enricher != nil {
        p.enricher.Write(ev)
    } else {
        p.l.Error("enricher is nil when writing event")
    }

    // Write the event to the external channel.
    if p.externalChannel != nil {
        select {
        case p.externalChannel <- ev:
        default:
            // Channel is full, drop the event.
            // We shouldn't slow down the reader.
            metrics.LostEventsCounter.WithLabelValues(utils.ExternalChannel, name).Inc()
        }
    }

    return nil
}
