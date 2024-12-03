// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package infiniband

import (
	"sync"

	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/plugin/api"
)

const (
	Name api.PluginName = "infiniband"
)

//go:generate mockgen -source=types_linux.go -destination=infiniband_mock_generated.go -package=infiniband
type infiniband struct {
	cfg       *kcfg.Config
	l         *log.ZapLogger
	isRunning bool
	startLock sync.Mutex
}

type CounterStat struct {
	Name   string
	Device string
	Port   string
}

type StatusParam struct {
	Name  string
	Iface string
}
