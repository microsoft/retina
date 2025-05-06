package ebpfwindows

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
	"unsafe"

	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	observer "github.com/cilium/cilium/pkg/hubble/observer/types"
	hp "github.com/cilium/cilium/pkg/hubble/parser"
	monitor "github.com/cilium/cilium/pkg/monitor"
	monitorapi "github.com/cilium/cilium/pkg/monitor/api"
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/log"
	metrics "github.com/microsoft/retina/pkg/metrics"
	"github.com/microsoft/retina/pkg/plugin/registry"
	"github.com/microsoft/retina/pkg/utils"
	"github.com/sirupsen/logrus"
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
	ErrNilEnricher = errors.New("enricher is nil")
)

// Plugin is the ebpfwindows plugin
type Plugin struct {
	l               *log.ZapLogger
	cfg             *kcfg.Config
	enricher        enricher.EnricherInterface
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
	parser, err := hp.New(logrus.WithField("windowsEbpf", "parser"),
		// We use noop getters here since we will use our own custom parser in hubble
		&NoopEndpointGetter,
		&NoopIdentityGetter,
		&NoopDNSGetter,
		&NoopIPGetter,
		&NoopServiceGetter,
		&NoopLinkGetter,
		&NoopPodMetadataGetter,
	)

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
	p.pullMetricsAndEvents(ctx)
	p.l.Info("Complete ebpfWindows plugin...")
	return nil
}

// metricsMapIterateCallback is the callback function that is called for each key-value pair in the metrics map.
func (p *Plugin) metricsMapIterateCallback(key *MetricsKey, value *MetricsValues) {
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
			metrics.DropBytesGauge.WithLabelValues(key.DropForwardReason(), egressLabel).Set(float64(value.BytesSum()))
			metrics.DropPacketsGauge.WithLabelValues(key.DropForwardReason(), egressLabel).Set(float64(value.Sum()))
		} else if key.IsIngress() {
			metrics.DropBytesGauge.WithLabelValues(key.DropForwardReason(), ingressLabel).Set(float64(value.BytesSum()))
			metrics.DropPacketsGauge.WithLabelValues(key.DropForwardReason(), ingressLabel).Set(float64(value.Sum()))
		}
	} else {
		p.l.Debug("MetricsMapIterateCallback Forward", zap.String("key", key.String()))
		if key.IsEgress() {
			metrics.ForwardPacketsGauge.WithLabelValues(egressLabel).Set(float64(value.Sum()))
			metrics.ForwardBytesGauge.WithLabelValues(egressLabel).Set(float64(value.BytesSum()))
		} else if key.IsIngress() {
			metrics.ForwardPacketsGauge.WithLabelValues(ingressLabel).Set(float64(value.Sum()))
			metrics.ForwardBytesGauge.WithLabelValues(ingressLabel).Set(float64(value.BytesSum()))
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
		return fmt.Errorf("error setting PATH environment variable: %v", err)
	}

	return nil
}

func (p *Plugin) pullMetricsAndEvents(ctx context.Context) {
	eventsMap := NewEventsMap()
	metricsMap := NewMetricsMap()

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
			eventsMap.UnregisterForCallback()
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
		return fmt.Errorf("invalid size %d", size)
	}

	if data == nil {
		return fmt.Errorf("handleTraceEvent data received is nil")
	}
	perfData := unsafe.Slice((*byte)(data), size)
	eventType := perfData[0]
	switch eventType {
	case monitorapi.MessageTypeDrop:
		if size <= uint32(unsafe.Sizeof(monitor.DropNotify{})) {
			return fmt.Errorf("invalid size for DropNotify %d", size)
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
		utils.AddPacketSize(meta, size-uint32(unsafe.Sizeof(monitor.DropNotify{})))
		fl := e.GetFlow()
		if fl == nil {
			return fmt.Errorf("dropnotify flow object is nil")
		}
		if fl.GetEventType() == nil {
			return fmt.Errorf("dropnotify event type is nil")
		}
		// Set the drop reason.
		eventType := fl.GetEventType().GetSubType()
		meta.DropReason = utils.DropReason(eventType)
		utils.AddRetinaMetadata(fl, meta)
		p.enricher.Write(e)
	case monitorapi.MessageTypeTrace:
		if size <= uint32(unsafe.Sizeof(monitor.TraceNotify{})) {
			return fmt.Errorf("invalid size for TraceNotify %d", size)
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
		utils.AddPacketSize(meta, size-uint32(unsafe.Sizeof(monitor.TraceNotify{})))
		fl := e.GetFlow()
		if fl == nil {
			return fmt.Errorf("tracenotify flow object is nil")
		}
		utils.AddRetinaMetadata(fl, meta)
		p.enricher.Write(e)
	}
	return nil
}
