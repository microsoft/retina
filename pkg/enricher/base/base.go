// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package base

import (
	"context"
	"reflect"
	"sync"

	"github.com/cilium/cilium/api/v1/flow"
	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	"github.com/cilium/cilium/pkg/hubble/container"
	"github.com/microsoft/retina/pkg/common"
	c "github.com/microsoft/retina/pkg/controllers/cache"
	"go.uber.org/zap"

	"github.com/microsoft/retina/pkg/log"
)

var (
	E           EnricherInterface
	Once        sync.Once
	Initialized bool
)

type Enricher struct {
	// ctx is the context of enricher
	Ctx context.Context

	// l is the logger
	L *log.ZapLogger

	// // cache is the cache of all the objects
	Cache c.CacheInterface

	InputRing *container.Ring

	OutputRing *container.Ring

	Reader *container.RingReader
}

func NewEnricher(ctx context.Context, logger *log.ZapLogger, cache c.CacheInterface) *Enricher {
	ir := container.NewRing(container.Capacity1023)
	return &Enricher{
		Ctx:        ctx,
		L:          logger,
		Cache:      cache,
		InputRing:  ir,
		OutputRing: container.NewRing(container.Capacity1023),
		Reader:     container.NewRingReader(ir, ir.OldestWrite()),
	}
}

func Instance() EnricherInterface {
	return E
}

func IsInitialized() bool {
	return Initialized
}

func (e *Enricher) Run(enrichFn func(ev *v1.Event)) {
	go func() {
		for {
			select {
			case <-e.Ctx.Done():
				e.L.Debug("context is done for enricher")
				return
			default:
				ev := e.Reader.NextFollow(e.Ctx)
				// nolint:gocritic
				// if err != nil {
				// se.L.Error("error while reading from input channel for enricher", zap.Error(err))
				// 	continue
				// }
				if ev == nil {
					e.L.Debug("received nil from input channel for enricher")
					continue
				}
				// todo
				switch fl := ev.Event.(type) {
				case *flow.Flow:
					e.L.Debug("Enriching flow", zap.Any("flow", fl))
					enrichFn(ev)
				default:
					e.L.Debug("received unknown type from input channel for enricher",
						zap.Any("obj", ev),
						zap.Any("type", reflect.TypeOf(ev)),
					)
				}
			}
		}
	}()
}

// export forwards the flow to other modules
func (e *Enricher) Export(ev *v1.Event) {
	e.OutputRing.Write(ev)
}

func (e *Enricher) GetEndpoint(obj interface{}) *flow.Endpoint {
	if obj == nil {
		return nil
	}

	switch o := obj.(type) {
	case *common.RetinaEndpoint:
		// TODO add service type
		return &flow.Endpoint{
			Namespace: o.Namespace(),
			PodName:   o.Name(),
			Labels:    o.FormattedLabels(),
			Workloads: e.getWorkloads(o.OwnerRefs()),
		}

	case *common.RetinaSvc:
		// todo
		return nil

	default:
		e.L.Debug("received unknown type from cache", zap.Any("obj", obj), zap.Any("type", reflect.TypeOf(obj)))
		return nil
	}
}

func (e *Enricher) getWorkloads(ownerRefs []*common.OwnerReference) []*flow.Workload {
	if ownerRefs == nil {
		return nil
	}
	workloads := make([]*flow.Workload, 0)

	for _, ownerRef := range ownerRefs {
		w := &flow.Workload{
			Name: ownerRef.Name,
			Kind: ownerRef.Kind,
		}

		workloads = append(workloads, w)
	}

	return workloads
}

func (e *Enricher) Write(ev *v1.Event) {
	e.InputRing.Write(ev)
}

func (e *Enricher) ExportReader() *container.RingReader {
	return container.NewRingReader(e.OutputRing, e.OutputRing.OldestWrite())
}
