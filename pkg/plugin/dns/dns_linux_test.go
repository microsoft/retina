// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package dns

import (
	"context"
	"reflect"
	"testing"

	"github.com/cilium/cilium/api/v1/flow"
	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	"github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/utils"
	"go.uber.org/mock/gomock"
	"gotest.tools/v3/assert"
)

func TestNew(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	cfg := &config.Config{
		EnablePodLevel: true,
	}
	d := New(cfg)
	assert.Assert(t, d != nil)
	assert.Equal(t, d.Name(), name)
}

func TestStop(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	cfg := &config.Config{
		EnablePodLevel: true,
	}
	d := &dns{
		cfg: cfg,
		l:   log.Logger().Named(name),
	}
	// Should not panic when not running.
	err := d.Stop()
	assert.NilError(t, err)
}

func TestSetupChannel(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	cfg := &config.Config{
		EnablePodLevel: true,
	}
	d := &dns{
		cfg: cfg,
		l:   log.Logger().Named(name),
	}

	ch := make(chan *v1.Event, 10)
	err := d.SetupChannel(ch)
	assert.NilError(t, err)
	assert.Assert(t, d.externalChannel == ch)
}

func TestGenerate(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	d := &dns{
		cfg: &config.Config{},
		l:   log.Logger().Named(name),
	}
	err := d.Generate(context.Background())
	assert.NilError(t, err)
}

// Helpers for testing event matching.

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
