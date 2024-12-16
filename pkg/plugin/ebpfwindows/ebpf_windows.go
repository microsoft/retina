package ebpfwindows

import (
	"context"
	"time"

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
	p.pullCiliumMetrics(ctx)
	return nil
}

// metricsMapIterateCallback is the callback function that is called for each key-value pair in the metrics map.
func (p *Plugin) metricsMapIterateCallback(key *MetricsKey, value *MetricsValues) {
	p.l.Info("MetricsMapIterateCallback")
	p.l.Info("Key", zap.String("Key", key.String()))
	p.l.Info("Value", zap.String("Value", value.String()))
}

// pullCiliumeBPFMetrics is the function that is called periodically by the timer.
func (p *Plugin) pullCiliumMetrics(ctx context.Context) {

	ticker := time.NewTicker(p.cfg.MetricsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			metricsMap := NewMetricsMap()
			err := metricsMap.IterateWithCallback(p.metricsMapIterateCallback)
			if err != nil {
				p.l.Error("Error iterating metrics map", zap.Error(err))
			}
		case <-ctx.Done():
			p.l.Error("ebpfwindows plugin canceling", zap.Error(ctx.Err()))
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
