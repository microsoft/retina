package ebpfwindows

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"
	"unsafe"

	plugincommon "github.com/microsoft/retina/pkg/plugin/common"

	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	observer "github.com/cilium/cilium/pkg/hubble/observer/types"
	monitorAPI "github.com/cilium/cilium/pkg/monitor/api"
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/log"
	metrics "github.com/microsoft/retina/pkg/metrics"
	"github.com/microsoft/retina/pkg/plugin/registry"
	"github.com/microsoft/retina/pkg/utils"
	"go.uber.org/zap"
)

const (
	// name of the ebpfwindows plugin
	name string = "ebpfwindows"
	// metrics direction
	ingressLabel = "ingress"
	egressLabel  = "egress"
)

var (
	errInvalidSize             = errors.New("invalid size")
	errNilHandleTraceEventData = errors.New("handleTraceEvent data received is nil")
	errNilDropNotifyFlow       = errors.New("dropnotify flow object is nil")
	errNilDropNotifyEvent      = errors.New("dropnotify event type is nil")
	errInvalidDropNotifySize   = errors.New("invalid size for DropNotify")
	errInvalidTraceNotifySize  = errors.New("invalid size for TraceNotify")
	errNilTraceNotifyFlow      = errors.New("tracenotify flow object is nil")
)

// Plugin is the ebpfwindows plugin
type Plugin struct {
	l               *log.ZapLogger
	cfg             *kcfg.Config
	enricher        enricher.EnricherInterface
	externalChannel chan *v1.Event
	parser          *Parser
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
	parser, err := NewParser(slog.Default().With("WindowsEbpf", "parser"))
	if err != nil {
		p.l.Error("Failed to create parser", zap.Error(err))
		return fmt.Errorf("failed to create parser: %w", err)
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

	ciliumEnabled, err := plugincommon.IsCiliumOnWindowsEnabled()

	if err != nil {
		p.l.Error("Error while checking if Cilium is enabled on Windows", zap.Error(err))
		return fmt.Errorf("Failed to check if Cilium is enabled on Windows: %w", err)
	}

	if !ciliumEnabled {
		p.l.Warn("Cilium is not enabled on Windows, skipping ebpfWindows plugin initialization")
		return nil
	}

	p.l.Info("Cilium is enabled on Windows, proceeding with ebpfWindows plugin initialization")
	p.pullMetricsAndEvents(ctx)
	p.l.Info("Complete ebpfWindows plugin...")
	return nil
}

// metricsMapIterateCallback is the callback function that is called for each key-value pair in the metrics map.
func (p *Plugin) metricsMapIterateCallback(key *MetricsKey, value *MetricsValue) {
	if key == nil {
		p.l.Error("MetricsMapIterateCallback key is nil")
		return
	}
	if value == nil {
		p.l.Error("MetricsMapIterateCallback value is nil")
		return
	}
	if key.IsDrop() {
		p.l.Debug("MetricsMapIterateCallback Drop", zap.String("key", key.String()))
		if key.IsEgress() {
			metrics.DropBytesGauge.WithLabelValues(key.DropForwardReason(), egressLabel).Set(float64(value.Bytes))
			metrics.DropPacketsGauge.WithLabelValues(key.DropForwardReason(), egressLabel).Set(float64(value.Count))
		} else if key.IsIngress() {
			metrics.DropBytesGauge.WithLabelValues(key.DropForwardReason(), ingressLabel).Set(float64(value.Bytes))
			metrics.DropPacketsGauge.WithLabelValues(key.DropForwardReason(), ingressLabel).Set(float64(value.Count))
		} else {
			p.l.Error("MetricsMapIterateCallback drop key is neither ingress nor egress", zap.String("key", key.String()))
		}
	} else {
		p.l.Debug("MetricsMapIterateCallback Forward", zap.String("key", key.String()))
		if key.IsEgress() {
			metrics.ForwardPacketsGauge.WithLabelValues(egressLabel).Set(float64(value.Count))
			metrics.ForwardBytesGauge.WithLabelValues(egressLabel).Set(float64(value.Bytes))
		} else if key.IsIngress() {
			metrics.ForwardPacketsGauge.WithLabelValues(ingressLabel).Set(float64(value.Count))
			metrics.ForwardBytesGauge.WithLabelValues(ingressLabel).Set(float64(value.Bytes))
		} else {
			p.l.Error("MetricsMapIterateCallback forward key is neither ingress nor egress", zap.String("key", key.String()))
		}
	}
}

// eventsMapCallback is the callback function that is called for each value  in the events map.
func (p *Plugin) eventsMapCallback(data unsafe.Pointer, size uint32) {
	err := p.handleTraceEvent(data, size)
	if err != nil {
		p.l.Error("Error handling trace event", zap.Error(err))
	}
}

func (p *Plugin) addEbpfToPath() error {
	currPath := os.Getenv("PATH")
	if strings.Contains(currPath, "ebpf-for-windows") {
		return nil
	}
	programFiles := os.Getenv("ProgramFiles")
	ebpfWindowsPath := programFiles + "\\ebpf-for-windows\\"
	newPath := currPath + ";" + ebpfWindowsPath
	if err := os.Setenv("PATH", newPath); err != nil {
		p.l.Error("Error setting PATH environment variable", zap.Error(err))
		return fmt.Errorf("failed to set PATH environment variable: %w", err)
	}

	return nil
}

func (p *Plugin) pullMetricsAndEvents(ctx context.Context) {
	eventsMap := NewEventsMap()
	metricsMap := NewMetricsMap()
	prevLostEventsCount := uint64(0)

	err := p.addEbpfToPath()
	if err != nil {
		return
	}

	if enricher.IsInitialized() && p.cfg.EnablePodLevel {
		p.enricher = enricher.Instance()
	} else {
		p.l.Warn("retina enricher is not initialized")
	}

	if p.enricher != nil {
		err := eventsMap.RegisterForCallback(p.l, p.eventsMapCallback)
		if err != nil {
			p.l.Error("Error registering for events map callback", zap.Error(err))
			return
		}

		defer func() {
			p.l.Error("ebpfwindows plugin canceling", zap.Error(ctx.Err()))
			err := eventsMap.UnregisterForCallback()
			if err != nil {
				p.l.Error("Error unregistering events map callback", zap.Error(err))
			}
		}()
	}

	ticker := time.NewTicker(p.cfg.MetricsInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			err := metricsMap.IterateWithCallback(p.l, p.metricsMapIterateCallback)
			if err != nil {
				p.l.Error("Error iterating metrics map", zap.Error(err))
			}

			lostEventsCount, err := GetLostEventsCount()

			if err != nil {
				p.l.Error("Error getting lost events count", zap.Error(err))
			} else {
				// The lost events count is cumulative, so we need to calculate the difference
				if lostEventsCount > prevLostEventsCount {
					counterToAdd := lostEventsCount - prevLostEventsCount
					metrics.LostEventsCounter.WithLabelValues(utils.Kernel, name).Add(float64(counterToAdd))
					prevLostEventsCount = lostEventsCount
				}
			}

		case <-ctx.Done():
			p.l.Error("ebpfwindows plugin canceling", zap.Error(ctx.Err()))
			err := eventsMap.UnregisterForCallback()
			if err != nil {
				p.l.Error("Error Unregistering Events Map callback", zap.Error(err))
			}
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

func (p *Plugin) handleTraceEvent(data unsafe.Pointer, size uint32) error {
	if uintptr(size) < unsafe.Sizeof(uint8(0)) {
		return fmt.Errorf("%w: %d", errInvalidSize, size)
	}

	if data == nil {
		return fmt.Errorf("%w", errNilHandleTraceEventData)
	}
	perfData := unsafe.Slice((*byte)(data), size)
	eventType := perfData[0]
	switch eventType {
	case monitorAPI.MessageTypeDrop:
		if size <= uint32(unsafe.Sizeof(DropNotify{})) {
			return fmt.Errorf("%w: %d", errInvalidDropNotifySize, size)
		}

		e, err := p.parser.Decode(&observer.MonitorEvent{
			Payload: &observer.PerfEvent{
				Data: perfData,
			},
		})
		if err != nil {
			return fmt.Errorf("could not convert dropnotify event to flow: %w", err)
		}
		meta := &utils.RetinaMetadata{}
		utils.AddPacketSize(meta, size-uint32(unsafe.Sizeof(DropNotify{})))
		fl := e.GetFlow()
		if fl == nil {
			return fmt.Errorf("%w", errNilDropNotifyFlow)
		}
		if fl.GetEventType() == nil {
			return fmt.Errorf("%w", errNilDropNotifyEvent)
		}
		// Set the drop reason.
		eventType := fl.GetEventType().GetSubType()
		meta.DropReason = utils.DropReason(eventType)
		utils.AddRetinaMetadata(fl, meta)
		p.enricher.Write(e)
	case monitorAPI.MessageTypeTrace:
		e := &v1.Event{}
		if size <= uint32(unsafe.Sizeof(TraceNotify{})) {
			return fmt.Errorf("%w: %d", errInvalidTraceNotifySize, size)
		}
		e, err := p.parser.Decode(&observer.MonitorEvent{
			Payload: &observer.PerfEvent{
				Data: perfData,
			},
		})
		if err != nil {
			return fmt.Errorf("could not convert tracenotify event to flow: %w", err)
		}
		meta := &utils.RetinaMetadata{}
		utils.AddPacketSize(meta, size-uint32(unsafe.Sizeof(TraceNotify{})))
		fl := e.GetFlow()
		if fl == nil {
			return fmt.Errorf("%w", errNilTraceNotifyFlow)
		}
		utils.AddRetinaMetadata(fl, meta)
		p.enricher.Write(e)
	}
	return nil
}
