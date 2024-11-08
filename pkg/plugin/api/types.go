// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

// Package api provides the api for all Retina eBPF plugins.
package api

import (
	"context"

	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
)

//go:generate mockgen -destination=mock/mock_plugin.go -copyright_file=../../lib/ignore_headers.txt -package=mock github.com/microsoft/retina/pkg/plugin/api Plugin

const (
	Meter       string = "retina-meter"
	ServiceName string = "retina"
)

// PluginName provides the type for the name of the plugin.
type PluginName string

// Plugin provides the interface that all Retina eBPF plugins must implement.
type Plugin interface {
	// Name returns the name of the plugin
	Name() string
	// Generate generates the plugin specific header files.
	// This maybe no-op for plugins that don't use eBPF.
	Generate(ctx context.Context) error
	// Compile compiles the eBPF code to generate bpf object.
	// This maybe no-op for plugins that don't use eBPF.
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
