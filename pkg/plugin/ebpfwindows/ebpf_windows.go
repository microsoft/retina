package ebpfwindows

import (
    "context"
    "errors"
    "time"
    "unsafe"

    v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
    kcfg "github.com/microsoft/retina/pkg/config"
    "github.com/microsoft/retina/pkg/log"
    "github.com/microsoft/retina/pkg/plugin/registry"
    "go.uber.org/zap"
)

const (
    // name of the ebpfwindows plugin
    name = "windowseBPF"
)

var (
    ErrInvalidEventData = errors.New("The Cilium Event Data is invalid")
)

// Plugin is the ebpfwindows plugin
type Plugin struct {
    l   *log.ZapLogger
    cfg *kcfg.Config
}

func New(cfg *kcfg.Config) registry.Plugin {
    return &Plugin{
        l:   log.Logger().Named(name),
        cfg: cfg,
    }
}

// Init is a no-op for the ebpfwindows plugin
func (p *Plugin) Init() error {
    return nil
}

// Name returns the name of the ebpfwindows plugin
func (p *Plugin) Name() string {
    return name
}

// Start the plugin by starting a periodic timer.
func (p *Plugin) Start(ctx context.Context) error {

    p.l.Info("Start ebpfWindows plugin...")
    p.pullCiliumMetricsAndEvents(ctx)
    return nil
}

// metricsMapIterateCallback is the callback function that is called for each key-value pair in the metrics map.
func (p *Plugin) metricsMapIterateCallback(key *MetricsKey, value *MetricsValues) {
    p.l.Info("MetricsMapIterateCallback")
    p.l.Info("Key", zap.String("Key", key.String()))
    p.l.Info("Value", zap.String("Value", value.String()))
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

// SetupChannel is a no-op for the ebpfwindows plugin
func (p *Plugin) SetupChannel(ch chan *v1.Event) error {
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

    return nil
}
