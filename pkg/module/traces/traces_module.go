// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package traces

import (
	"context"
	"sync"
	"time"

	api "github.com/microsoft/retina/crd/api/v1alpha1"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/pubsub"
	"go.uber.org/zap"
)

var (
	t    *Module
	once sync.Once
)

const (
	moduleIntervalSecs = 1 * time.Second
)

type Module struct {
	*sync.RWMutex
	// ctx is the context of the trace module
	ctx context.Context

	// l is the logger
	l *log.ZapLogger

	// traceConfigs is the list of trace configurations from CRD
	configs []*api.TraceConfiguration

	// traceOutputCOnfigs is the list of trace output configurations from CRD
	outputConfigs []*api.TraceOutputConfiguration

	// pubsub is the pubsub client
	pubsub pubsub.PubSubInterface

	// isRunning is the flag to indicate if the trace module is running
	isRunning bool

	// TODO add the filter manager here or add a pubsub topic for
	// filter manager to subscribe to
}

func NewModule(ctx context.Context, pubsub pubsub.PubSubInterface) *Module {
	// this is a thread-safe singleton instance of the trace module
	once.Do(func() {
		t = &Module{
			RWMutex:       &sync.RWMutex{},
			l:             log.Logger().Named(string("TraceModule")),
			ctx:           ctx,
			pubsub:        pubsub,
			configs:       make([]*api.TraceConfiguration, 0),
			outputConfigs: make([]*api.TraceOutputConfiguration, 0),
		}

		t.init()
	})

	return t
}

func (t *Module) init() {
	// TODO
}

func (t *Module) Run() {
	if t.isRunning {
		return
	}
	go func() {
		t.isRunning = true
		ticker := time.NewTicker(moduleIntervalSecs)
		defer ticker.Stop()

		for {
			select {
			case <-t.ctx.Done():
				return
			case <-ticker.C:
				if err := t.run(); err != nil {
					t.l.Error("error running trace module", zap.Error(err))
				}
			}
		}
	}()
}

func (t *Module) Reconcile(spec *api.TracesSpec) error {
	return nil
}

func (t *Module) run() error {
	return nil
}
