// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package enricher

import (
	"context"
	"errors"
	"io"
	"reflect"
	"sync"

	"time"

	"github.com/cilium/cilium/api/v1/flow"
	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	"github.com/cilium/cilium/pkg/hubble/container"
	"github.com/microsoft/retina/pkg/common"
	"github.com/microsoft/retina/pkg/controllers/cache"
	"github.com/microsoft/retina/pkg/log"
	"go.uber.org/zap"
)

var (
	e           *Enricher
	once        sync.Once
	initialized bool
)

type Enricher struct {
	// ctx is the context of enricher
	ctx context.Context

	// l is the logger
	l *log.ZapLogger

	// cache is the cache of all the objects
	cache cache.CacheInterface

	inputRing *container.Ring

	Reader *container.RingReader

	outputRing *container.Ring
}

func New(ctx context.Context, cache cache.CacheInterface) *Enricher {
	once.Do(func() {
		ir := container.NewRing(container.Capacity1023)
		e = &Enricher{
			ctx:        ctx,
			l:          log.Logger().Named("enricher"),
			cache:      cache,
			inputRing:  ir,
			Reader:     container.NewRingReader(ir, ir.OldestWrite()),
			outputRing: container.NewRing(container.Capacity1023),
		}
		initialized = true
	})

	return e
}

func Instance() *Enricher {
	return e
}

func IsInitialized() bool {
	return initialized
}

func (e *Enricher) Run() {
	go func() {
		for {
			select {
			case <-e.ctx.Done():
				e.l.Debug("context is done for enricher")
				return
			default:
				ev := e.Reader.NextFollow(e.ctx)
				//if err != nil {
				//e.l.Error("error while reading from input channel for enricher", zap.Error(err))
				//	continue
				//}
				if ev == nil {
					e.l.Debug("received nil from input channel for enricher")
					continue
				}
				// todo
				switch ev.Event.(type) {
				case *flow.Flow:
					e.l.Debug("Enriching flow", zap.Any("flow", ev.Event.(*flow.Flow)))
					e.enrich(ev)
				default:
					e.l.Debug("received unknown type from input channel for enricher",
						zap.Any("obj", ev),
						zap.Any("type", reflect.TypeOf(ev)),
					)
				}
			}
		}
	}()
}

// enrich takes the flow and enriches it with the information from the cache
func (e *Enricher) enrich(ev *v1.Event) {
	flow := ev.Event.(*flow.Flow)

	// IPversion is a enum in the flow proto
	// 0: IPVersion_IP_NOT_USED
	// 1: IPVersion_IPv4
	// 2: IPVersion_IPv6
	if flow.IP.IpVersion > 1 {
		e.l.Error("IP version is not supported", zap.Any("IPVersion", flow.IP.IpVersion))
		return
	}
	if flow.IP.Source == "" {
		e.l.Debug("source IP is empty")
		return
	}
	srcObj := e.cache.GetObjByIP(flow.IP.Source)
	if srcObj != nil {
		flow.Source = e.getEndpoint(srcObj)
	}

	if flow.IP.Destination == "" {
		e.l.Debug("destination IP is empty")
		return
	}

	dstObj := e.cache.GetObjByIP(flow.IP.Destination)
	if dstObj != nil {
		flow.Destination = e.getEndpoint(dstObj)
	}

	ev.Event = flow
	e.l.Debug("enriched flow", zap.Any("flow", flow))
	e.export(ev)
}

// export forwards the flow to other modules
func (e *Enricher) export(ev *v1.Event) {
	e.outputRing.Write(ev)
}

func (e *Enricher) getEndpoint(obj interface{}) *flow.Endpoint {
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
		e.l.Debug("received unknown type from cache", zap.Any("obj", obj), zap.Any("type", reflect.TypeOf(obj)))
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
	e.inputRing.Write(ev)
}

func (e *Enricher) ExportReader() *container.RingReader {
	return container.NewRingReader(e.outputRing, e.outputRing.OldestWrite())
}

func (e *Enricher) Status() (float64, error) {
	rate, err := e.getFlowRate(e.outputRing, time.Now())
	if err != nil {
		e.l.Error("failed to get flow rate %w", zap.Error(err))
		return 0, err
	}
	return rate, nil
}

// ref: "getFlowRate" "github.com/cilium/cilium/pkg/hubble/observer/local_observer.go"
func (en *Enricher) getFlowRate(ring *container.Ring, at time.Time) (float64, error) {
	reader := container.NewRingReader(ring, ring.LastWriteParallel())
	count := 0
	since := at.Add(-1 * time.Minute)
	var lastSeenEvent *v1.Event
	for {
		e, err := reader.Previous()
		lost := e.GetLostEvent()
		if lost != nil {
			// a lost event means we read the complete ring buffer
			// if we read at least one flow, update `since` to calculate the rate over the available time range
			if lastSeenEvent != nil {
				since = lastSeenEvent.Timestamp.AsTime()
			}
			break
		} else if errors.Is(err, io.EOF) {
			// an EOF error means the ring buffer is empty, ignore error and continue
			break
		} else if err != nil {
			// unexpected error
			return 0, err
		}
		if _, isFlowEvent := e.Event.(*flow.Flow); !isFlowEvent {
			// ignore non flow events
			continue
		}
		if err := e.Timestamp.CheckValid(); err != nil {
			return 0, err
		}
		ts := e.Timestamp.AsTime()
		if ts.Before(since) {
			// scanned the last minute, exit loop
			break
		}
		lastSeenEvent = e
		count++
	}
	fl := float64(count) / at.Sub(since).Seconds()
	return fl, nil
}
