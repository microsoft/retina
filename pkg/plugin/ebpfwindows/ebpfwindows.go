// Package ebpfwindows is a WIP Retina plugin for Windows that sources network
// telemetry from the eBPF-for-Windows data plane (and the WCN/Cilium-on-Windows
// observability API) rather than the legacy HNS/VFP (hnsstats) path.
//
// STATUS: Work-in-progress skeleton. It follows the Retina registry.Plugin
// contract and compiles, but does not yet wire a live eBPF-for-Windows consumer.
// Intended data flow:
//
//	eBPF-for-Windows maps / ring buffers
//	    -> wcnagent / Microsoft.Wcn.Observability.eBPF adapter
//	    -> this plugin (Start) -> Retina flow objects -> metrics/control-plane
//
// See docs/03-Metrics/plugins/Windows/ebpfwindows.md for the proposal.
package ebpfwindows

import (
	"context"

	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/plugin/registry"
)

const name = "ebpfwindows"

func init() {
	// Self-register so pluginmanager can look it up by name via registry.Get.
	registry.Add(name, New)
}

// New returns the plugin. It is the plugin.PluginFunc for this plugin.
func New(*kcfg.Config) registry.Plugin {
	return &Plugin{l: log.Logger().Named(name)}
}

// Plugin consumes eBPF-for-Windows observability events on a Windows node.
type Plugin struct {
	l          *log.ZapLogger
	enricher   enricher.EnricherInterface
	eventChan  chan *v1.Event
	startedCtx context.Context
	cancel     context.CancelFunc
}

// Name returns the plugin name registered in the allowed plugins list.
func (p *Plugin) Name() string { return name }

// Generate is a no-op: eBPF-for-Windows programs are provided by the OS
// extension/WCN dataplane, not generated at build time by Retina.
func (p *Plugin) Generate(_ context.Context) error {
	p.l.Info("ebpfwindows Generate: no-op (programs provided by eBPF-for-Windows)")
	return nil
}

// Compile is a no-op for the same reason as Generate.
func (p *Plugin) Compile(_ context.Context) error {
	p.l.Info("ebpfwindows Compile: no-op (programs provided by eBPF-for-Windows)")
	return nil
}

// Init prepares plugin state. No host-side setup is required for the skeleton.
func (p *Plugin) Init() error {
	p.eventChan = make(chan *v1.Event, 1024)
	p.l.Info("ebpfwindows initialized")
	return nil
}

// Start begins consuming eBPF-for-Windows / WCN observability events.
//
// NOTE(WIP): the concrete consumer is not implemented. To finish it:
//  1. Register/enumerate the eBPF-for-Windows hook or the WCN observability API
//     (Microsoft.Wcn.Observability.eBPF.Retina) on this node.
//  2. Read maps/ring-buffer events as they arrive.
//  3. Convert each event to a cilium v1.Event (flow) carrying the 5-tuple,
//     direction, verdict (forward/drop), and duration, then send on eventChan.
//  4. Optionally hand flows to p.enricher for Kubernetes context before sending.
func (p *Plugin) Start(ctx context.Context) error {
	p.l.Info("ebpfwindows Start")
	p.enricher = enricher.Instance()
	p.startedCtx, p.cancel = context.WithCancel(ctx)
	return nil
}

// SetupChannel wires a downstream channel that receives the flows this plugin
// emits (e.g. the metrics pipeline or the Hubble observer).
func (p *Plugin) SetupChannel(_ chan *v1.Event) error {
	p.l.Info("ebpfwindows SetupChannel")
	return nil
}

// Stop tears down the consumer loop and the downstream channel.
func (p *Plugin) Stop() error {
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
	p.l.Info("ebpfwindows stopped")
	return nil
}
