// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package metrics

import (
	api "github.com/microsoft/retina/crd/api/v1alpha1"
	"github.com/microsoft/retina/pkg/log"
)

type baseMetricObject struct {
	advEnable   bool
	contextMode enrichmentContext
	ctxOptions  *api.MetricsContextOptions
	srcCtx      ContextOptionsInterface
	dstCtx      ContextOptionsInterface
	l           *log.ZapLogger
}

func newBaseMetricsObject(ctxOptions *api.MetricsContextOptions, fl *log.ZapLogger, isLocalContext enrichmentContext) baseMetricObject {
	b := baseMetricObject{
		advEnable:   ctxOptions.IsAdvanced(),
		ctxOptions:  ctxOptions,
		l:           fl,
		contextMode: isLocalContext,
	}

	b.populateCtxOptions(ctxOptions)
	return b
}

func (b *baseMetricObject) populateCtxOptions(ctxOptions *api.MetricsContextOptions) {
	if b.isLocalContext() {
		// when localcontext is enabled, we do not need the context options for both src and dst
		// metrics aggregation will be on a single pod basis and not the src/dst pod combination basis.
		// so we can getaway with just one context type. For this reason we will only use srccontext
		// we can ignore adding destination context.
		if ctxOptions.SourceLabels != nil {
			b.srcCtx = NewCtxOption(ctxOptions.SourceLabels, localCtx)
		}
	} else {
		if ctxOptions.SourceLabels != nil {
			b.srcCtx = NewCtxOption(ctxOptions.SourceLabels, source)
		}

		if ctxOptions.DestinationLabels != nil {
			b.dstCtx = NewCtxOption(ctxOptions.DestinationLabels, destination)
		}
	}
}

func (b *baseMetricObject) isLocalContext() bool {
	return b.contextMode == localContext
}
