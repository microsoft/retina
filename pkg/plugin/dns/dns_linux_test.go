// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//nolint:typecheck
package dns

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/cilium/cilium/api/v1/flow"
	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	"github.com/inspektor-gadget/inspektor-gadget/pkg/gadgets/trace/dns/types"
	"github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/controllers/cache"
	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/metrics"
	"github.com/microsoft/retina/pkg/plugin/common/mocks"
	"github.com/microsoft/retina/pkg/pubsub"
	"github.com/microsoft/retina/pkg/utils"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"go.uber.org/mock/gomock"
	"gotest.tools/v3/assert"
)

func TestStop(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	d := &dns{
		l:   log.Logger().Named(string(Name)),
		pid: 1234,
	}
	// Check nil tracer.
	d.Stop()

	// Check with tracer.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	m := mocks.NewMockITracer(ctrl)
	m.EXPECT().Detach(d.pid).Return(nil).Times(1)
	m.EXPECT().Close().Times(1)
	d.tracer = m
	d.Stop()
}

func TestStart(t *testing.T) {
	ctxTimeout, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	log.SetupZapLogger(log.GetDefaultLogOpts())

	c := cache.New(pubsub.New())
	e := enricher.New(ctxTimeout, c)
	e.Run()
	defer e.Reader.Close()

	d := &dns{
		l:   log.Logger().Named(string(Name)),
		pid: 1234,
		cfg: &config.Config{
			EnablePodLevel: true,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	m := mocks.NewMockITracer(ctrl)
	m.EXPECT().Attach(d.pid).Return(nil).Times(1)
	d.tracer = m
	err := d.Start(ctxTimeout)
	assert.Equal(t, err, nil)
	if d.enricher == nil {
		t.Fatal("enricher is nil")
	}

	// Test error case.
	expected := errors.New("Error")
	m = mocks.NewMockITracer(ctrl)
	m.EXPECT().Attach(d.pid).Return(expected).Times(1)
	d.tracer = m

	err = d.Start(ctxTimeout)
	assert.Error(t, err, expected.Error())
}

func TestMalformedEventHandler(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	d := &dns{
		l: log.Logger().Named(string(Name)),
	}

	// Test nil event.
	m = nil
	d.eventHandler(nil)
	assert.Equal(t, m, nil)

	// Test event with no Query type.
	m = nil
	event := &types.Event{
		Qr: "Z",
	}
	d.eventHandler(event)
	assert.Equal(t, m, nil)
}

func TestRequestEventHandler(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	metrics.InitializeMetrics()

	d := &dns{
		l: log.Logger().Named(string(Name)),
		cfg: &config.Config{
			EnablePodLevel: true,
		},
	}

	// Test event with Query type.
	m = nil
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	event := &types.Event{
		Qr:         "Q",
		Rcode:      "No Error",
		QType:      "A",
		DNSName:    "test.com",
		Addresses:  []string{},
		NumAnswers: 0,
		PktType:    "OUTGOING",
		SrcIP:      "1.1.1.1",
		DstIP:      "2.2.2.2",
		SrcPort:    58,
		DstPort:    8080,
		Protocol:   "TCP",
	}
	c := prometheus.NewCounter(prometheus.CounterOpts{})

	// Basic metrics.
	mockCV := metrics.NewMockCounterVec(ctrl)
	mockCV.EXPECT().WithLabelValues().Return(c).Times(1)
	before := value(c)
	metrics.DNSRequestCounter = mockCV

	// Advanced metrics.
	mockEnricher := enricher.NewMockEnricherInterface(ctrl)
	mockEnricher.EXPECT().Write(EventMatched(
		utils.DNSType_QUERY, 0, event.DNSName, []string{event.QType}, 0, []string{},
	)).Times(1)
	d.enricher = mockEnricher

	d.eventHandler(event)
	after := value(c)
	assert.Equal(t, after-before, float64(1))
}

func TestResponseEventHandler(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	metrics.InitializeMetrics()

	d := &dns{
		l: log.Logger().Named(string(Name)),
		cfg: &config.Config{
			EnablePodLevel: true,
		},
	}

	// Test event with Query type.
	m = nil
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	event := &types.Event{
		Qr:         "R",
		Rcode:      "No Error",
		QType:      "A",
		DNSName:    "test.com",
		Addresses:  []string{"1.1.1.1", "2.2.2.2"},
		NumAnswers: 2,
		PktType:    "HOST",
		SrcIP:      "1.1.1.1",
		DstIP:      "2.2.2.2",
		SrcPort:    58,
		DstPort:    8080,
		Protocol:   "TCP",
	}

	// Basic metrics.
	c := prometheus.NewCounter(prometheus.CounterOpts{})
	mockCV := metrics.NewMockCounterVec(ctrl)
	mockCV.EXPECT().WithLabelValues().Return(c).Times(1)
	before := value(c)
	metrics.DNSResponseCounter = mockCV

	// Advanced metrics.
	mockEnricher := enricher.NewMockEnricherInterface(ctrl)
	mockEnricher.EXPECT().Write(EventMatched(
		utils.DNSType_RESPONSE, 0, event.DNSName, []string{event.QType}, 2, []string{"1.1.1.1", "2.2.2.2"},
	)).Times(1)
	d.enricher = mockEnricher

	d.eventHandler(event)
	after := value(c)
	assert.Equal(t, after-before, float64(1))
}

func value(c prometheus.Counter) float64 {
	m := &dto.Metric{}
	c.Write(m)

	return m.Counter.GetValue()
}

// Helpers.

type EventMatcher struct {
	qType      utils.DNSType
	rCode      uint32
	query      string
	qTypes     []string
	numAnswers uint32
	ips        []string
}

func (m *EventMatcher) Matches(x interface{}) bool {
	inputFlow := x.(*v1.Event).Event.(*flow.Flow)
	expectedDNS, expectedDNSType, expectedNumResponses := utils.GetDNS(inputFlow)
	return expectedDNS != nil &&
		expectedDNS.GetRcode() == m.rCode &&
		expectedDNS.GetQuery() == m.query &&
		reflect.DeepEqual(expectedDNS.GetIps(), m.ips) &&
		reflect.DeepEqual(expectedDNS.GetQtypes(), m.qTypes) &&
		expectedDNSType == m.qType &&
		expectedNumResponses == m.numAnswers
}

func (m *EventMatcher) String() string {
	return "is anything"
}

func EventMatched(qType utils.DNSType, rCode uint32, query string, qTypes []string, numAnswers uint32, ips []string) gomock.Matcher {
	return &EventMatcher{
		qType:      qType,
		rCode:      rCode,
		query:      query,
		qTypes:     qTypes,
		numAnswers: numAnswers,
		ips:        ips,
	}
}
