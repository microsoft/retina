// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package api

import (
	"context"

	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
)

const (
	Meter       string = "retina-meter"
	ServiceName string = "retina"
)

type PluginName string

// PluginEvent - Generic plugin event structure to receive data from plugin
type PluginEvent struct {
	Name PluginName
}

//go:generate go run github.com/golang/mock/mockgen@v1.6.0 -destination=mock_plugin.go -copyright_file=../../lib/ignore_headers.txt -package=api github.com/microsoft/retina/pkg/plugin/api Plugin

// All functions should be idempotent.
type Plugin interface {
	Name() string
	// Generate the plugin specific header files.
	// This maybe no-op for plugins that don't use eBPF.
	Generate(ctx context.Context) error
	// Compile the ebpf to generate bpf object.
	// This maybe no-op for plugins that don't use eBPF.
	Compile(ctx context.Context) error
	// Init initializes plugin specific objects. Plugin has to send data through the channel passed in arg.
	Init() error
	// Start The plugin has to start its execution in collecting traces
	Start(ctx context.Context) error
	// Stop The plugin has to stop its job
	Stop() error
	// Allow adding external channels that clients can use
	// to get data from the plugin. This allows custom post processing of plugin data.
	SetupChannel(chan *v1.Event) error
}
