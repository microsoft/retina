// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package dns

import (
	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/metrics"
	"github.com/microsoft/retina/pkg/plugin/api"
	"github.com/microsoft/retina/pkg/plugin/common"
)

const (
	Name api.PluginName = "dns"
)

var m metrics.ICounterVec

type dns struct {
	cfg             *kcfg.Config
	l               *log.ZapLogger
	tracer          common.ITracer
	pid             uint32
	externalChannel chan *v1.Event
}
