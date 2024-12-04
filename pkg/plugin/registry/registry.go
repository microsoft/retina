package registry

import (
	"context"

	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	kcfg "github.com/microsoft/retina/pkg/config"
)

const (
	Meter       string = "retina-meter"
	ServiceName string = "retina"
)

// Plugin provides the interface that all Retina plugins must implement.
type Plugin interface {
	// Name returns the name of the plugin.
	Name() string
	// Generate generates the plugin specific header files.
	// This may be no-op for plugins that don't use eBPF.
	Generate(ctx context.Context) error
	// Compile compiles the eBPF code to generate bpf object.
	// This may be no-op for plugins that don't use eBPF.
	Compile(ctx context.Context) error
	// Init initializes plugin specific objects. Depend on a given configuration, it may initialize eBPF maps, etc.
	Init() error
	// Start starts the plugin. The plugin should start its main loop and return.
	Start(ctx context.Context) error
	// Stop stops the plugin. The plugin should clean up all resources and exit.
	Stop() error
	// SetupChannel allows external components to setup a channel to the plugin to receive its events.
	// This can be useful for plugins that need to send data to other components for post-processing.
	SetupChannel(chan *v1.Event) error
}

// PluginFunc is the Constructor func that all PLugins must provide to Register.
type PluginFunc func(*kcfg.Config) Plugin

// Plugins is the centralized list of Retina plugins their New functions to create them.
var Plugins = map[string]PluginFunc{}

func Register(name string, f PluginFunc) {
	Plugins[name] = f
}
