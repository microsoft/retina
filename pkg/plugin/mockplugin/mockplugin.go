// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package mockplugin

import (
	"context"
	"fmt"

	hubblev1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/plugin/registry"
)

const name = "mockplugin"

const (
	initialize = iota + 1
	start
	stop
)

type MockPlugin struct {
	cfg   *kcfg.Config
	state int
	l     *log.ZapLogger
}

func init() {
	registry.Plugins[name] = New
}

// New creates a mock plugin.
func New(cfg *kcfg.Config) registry.Plugin {
	return &MockPlugin{
		cfg: cfg,
	}
}

func (mp *MockPlugin) Name() string {
	return "mockplugin"
}

func (mp *MockPlugin) Generate(context.Context) error {
	return nil
}

func (mp *MockPlugin) Compile(context.Context) error {
	return nil
}

func (mp *MockPlugin) Init() error {
	mp.state = initialize
	return nil
}

func (mp *MockPlugin) Start(context.Context) error {
	if mp.state != initialize {
		return fmt.Errorf("plugin not initialized")
	}
	mp.state = start
	return nil
}

func (mp *MockPlugin) Stop() error {
	if mp.state != start {
		return nil
	}
	mp.state = stop
	return nil
}

func (mp *MockPlugin) SetupChannel(chan *hubblev1.Event) error {
	return nil
}
