// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package tcpretrans

import (
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/plugin/api"
	gadgetcontext "github.com/inspektor-gadget/inspektor-gadget/pkg/gadget-context"
	"github.com/inspektor-gadget/inspektor-gadget/pkg/gadgets/trace/tcpretrans/tracer"
)

const (
	Name api.PluginName = "tcpretrans"
)

type tcpretrans struct {
	cfg       *kcfg.Config
	l         *log.ZapLogger
	tracer    *tracer.Tracer
	gadgetCtx *gadgetcontext.GadgetContext
	enricher  enricher.EnricherInterface
}
