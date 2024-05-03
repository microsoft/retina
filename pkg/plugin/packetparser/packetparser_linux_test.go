// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package packetparser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"runtime"
	"sync"
	"testing"
	"time"

	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/perf"
	"github.com/florianl/go-tc"
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/metrics"
	"github.com/microsoft/retina/pkg/plugin/packetparser/mocks"
	"github.com/microsoft/retina/pkg/watchers/endpoint"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
	"go.uber.org/mock/gomock"
)

var (
	cfgPodLevelEnabled = &kcfg.Config{
		EnablePodLevel:           true,
		BypassLookupIPOfInterest: true,
	}
	cfgPodLevelDisabled = &kcfg.Config{
		EnablePodLevel: false,
	}
)

func TestCleanAll(t *testing.T) {
	opts := log.GetDefaultLogOpts()
	log.SetupZapLogger(opts)

	p := &packetParser{
		cfg: cfgPodLevelEnabled,
		l:   log.Logger().Named("test"),
	}
	assert.Nil(t, p.cleanAll())

	p.tcMap = &sync.Map{}
	assert.Nil(t, p.cleanAll())

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mtcnl := mocks.NewMockITc(ctrl)
	mtcnl.EXPECT().Close().Return(nil).AnyTimes()

	mq := mocks.NewMockIQdisc(ctrl)
	mq.EXPECT().Delete(gomock.Any()).Return(nil).AnyTimes()

	getQdisc = func(tcnl ITc) IQdisc {
		return mq
	}

	p.tcMap.Store(key{
		name:         "test",
		hardwareAddr: "test",
		netNs:        1,
	}, &val{
		tcnl:         mtcnl,
		tcIngressObj: &tc.Object{},
		tcEgressObj:  &tc.Object{},
	})
	p.tcMap.Store(key{
		name:         "test2",
		hardwareAddr: "test2",
		netNs:        2,
	}, &val{
		tcnl:         mtcnl,
		tcIngressObj: &tc.Object{},
		tcEgressObj:  &tc.Object{},
	})

	assert.Nil(t, p.cleanAll())

	keyCount := 0
	p.tcMap.Range(func(k, v interface{}) bool {
		keyCount++
		return true
	})
	assert.Equal(t, 0, keyCount)
}

func TestClean(t *testing.T) {
	opts := log.GetDefaultLogOpts()
	log.SetupZapLogger(opts)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Test nil.
	p := &packetParser{
		cfg: cfgPodLevelEnabled,
		l:   log.Logger().Named("test"),
	}
	p.clean(nil, nil, nil) // Should not panic.

	// Test tcnl calls.
	mq := mocks.NewMockIQdisc(ctrl)
	mq.EXPECT().Delete(gomock.Any()).Return(nil).Times(2)

	mtcnl := mocks.NewMockITc(ctrl)
	mtcnl.EXPECT().Qdisc().Return(nil).Times(2)
	mtcnl.EXPECT().Close().Return(nil).Times(1)

	getQdisc = func(tcnl ITc) IQdisc {
		// Add this verify tcnl.Qdisc() is called twice
		tcnl.Qdisc()
		return mq
	}

	p.clean(mtcnl, &tc.Object{}, &tc.Object{})
}

func TestCleanWithErrors(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	p := &packetParser{
		cfg: cfgPodLevelEnabled,
		l:   log.Logger().Named("test"),
	}

	// Test we try delete qdiscs even if we get errors.
	mq := mocks.NewMockIQdisc(ctrl)
	mq.EXPECT().Delete(gomock.Any()).Return(errors.New("error")).Times(2)

	mtcnl := mocks.NewMockITc(ctrl)
	mtcnl.EXPECT().Qdisc().Return(nil).AnyTimes()
	mtcnl.EXPECT().Close().Return(nil).Times(1)

	getQdisc = func(tcnl ITc) IQdisc {
		return mq
	}

	p.clean(mtcnl, &tc.Object{}, &tc.Object{})
}

func TestEndpointWatcherCallbackFn_EndpointDeleted(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	p := &packetParser{
		cfg:              cfgPodLevelEnabled,
		l:                log.Logger().Named("test"),
		interfaceLockMap: &sync.Map{},
	}
	p.tcMap = &sync.Map{}
	linkAttr := netlink.LinkAttrs{
		Name:         "test",
		HardwareAddr: []byte("test"),
		NetNsID:      1,
	}
	key := ifaceToKey(linkAttr)
	p.tcMap.Store(key, &val{
		tcnl:         nil,
		tcIngressObj: &tc.Object{},
		tcEgressObj:  &tc.Object{},
	})

	// Create EndpointDeleted event.
	e := &endpoint.EndpointEvent{
		Type: endpoint.EndpointDeleted,
		Obj:  linkAttr,
	}

	p.endpointWatcherCallbackFn(e)

	_, ok := p.tcMap.Load(key)
	assert.False(t, ok)
}

func TestCreateQdiscAndAttach(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mfilter := mocks.NewMockIFilter(ctrl)
	mfilter.EXPECT().Add(gomock.Any()).Return(nil).AnyTimes()

	mq := mocks.NewMockIQdisc(ctrl)
	mq.EXPECT().Add(gomock.Any()).Return(nil).AnyTimes()

	mtcnl := mocks.NewMockITc(ctrl)
	mtcnl.EXPECT().Qdisc().Return(nil).AnyTimes()

	getQdisc = func(tcnl ITc) IQdisc {
		return mq
	}

	getFilter = func(tcnl ITc) IFilter {
		return mfilter
	}

	tcOpen = func(c *tc.Config) (ITc, error) {
		return mtcnl, nil
	}

	getFD = func(e *ebpf.Program) int {
		return 1
	}

	pObj := &packetparserObjects{} //nolint:typecheck
	pObj.EndpointIngressFilter = &ebpf.Program{}
	pObj.EndpointEgressFilter = &ebpf.Program{}

	p := &packetParser{
		cfg:              cfgPodLevelEnabled,
		l:                log.Logger().Named("test"),
		objs:             pObj,
		interfaceLockMap: &sync.Map{},
		endpointIngressInfo: &ebpf.ProgramInfo{
			Name: "ingress",
		},
		endpointEgressInfo: &ebpf.ProgramInfo{
			Name: "egress",
		},
		hostIngressInfo: &ebpf.ProgramInfo{
			Name: "ingress",
		},
		hostEgressInfo: &ebpf.ProgramInfo{
			Name: "egress",
		},
		tcMap: &sync.Map{},
	}
	linkAttr := netlink.LinkAttrs{
		Name:         "test",
		HardwareAddr: []byte("test"),
		NetNsID:      1,
	}
	// Test veth.
	p.createQdiscAndAttach(linkAttr, Veth)

	key := ifaceToKey(linkAttr)
	_, ok := p.tcMap.Load(key)
	assert.True(t, ok)

	pObj.HostIngressFilter = &ebpf.Program{}
	pObj.HostEgressFilter = &ebpf.Program{}
	linkAttr2 := netlink.LinkAttrs{
		Name:         "test2",
		HardwareAddr: []byte("test2"),
		NetNsID:      2,
	}
	// Test Device.
	p.createQdiscAndAttach(linkAttr2, Device)

	key = ifaceToKey(linkAttr2)
	_, ok = p.tcMap.Load(key)
	assert.True(t, ok)
}

func TestReadData_Error(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mperf := mocks.NewMockIPerf(ctrl)
	mperf.EXPECT().Read().Return(perf.Record{}, errors.New("error")).AnyTimes()

	menricher := enricher.NewMockEnricherInterface(ctrl) //nolint:typecheck
	menricher.EXPECT().Write(gomock.Any()).Times(0)

	p := &packetParser{
		cfg:    cfgPodLevelEnabled,
		l:      log.Logger().Named("test"),
		reader: mperf,
	}
	p.readData()

	// Lost samples.
	mperf.EXPECT().Read().Return(perf.Record{
		LostSamples: 1,
	}, nil).AnyTimes()
	p.readData()
}

func TestReadDataPodLevelEnabled(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bpfEvent := &packetparserPacket{ //nolint:typecheck
		SrcIp:   uint32(83886272), // 192.0.0.5
		DstIp:   uint32(16777226), // 10.0.0.1
		Proto:   uint8(6),         // TCP
		Dir:     uint32(1),        // TO Endpoint
		SrcPort: uint16(80),
		DstPort: uint16(443),
	}
	bytes, _ := json.Marshal(bpfEvent)
	record := perf.Record{
		LostSamples: 0,
		RawSample:   bytes,
	}

	mperf := mocks.NewMockIPerf(ctrl)
	mperf.EXPECT().Read().Return(record, nil).MinTimes(1)

	menricher := enricher.NewMockEnricherInterface(ctrl) //nolint:typecheck
	menricher.EXPECT().Write(gomock.Any()).MinTimes(1)

	p := &packetParser{
		cfg:            cfgPodLevelEnabled,
		l:              log.Logger().Named("test"),
		reader:         mperf,
		enricher:       menricher,
		recordsChannel: make(chan perf.Record, buffer),
	}

	mICounterVec := metrics.NewMockICounterVec(ctrl)
	mICounterVec.EXPECT().WithLabelValues(gomock.Any()).Return(prometheus.NewCounter(prometheus.CounterOpts{})).AnyTimes()

	metrics.LostEventsCounter = mICounterVec

	exCh := make(chan *v1.Event, 10)
	p.SetupChannel(exCh)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	p.run(ctx)

	// Test we get the event.
	select {
	case <-exCh:
	default:
		t.Fatal("Expected event in external channel, got none")
	}
}

func TestStartPodLevelDisabled(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	p := &packetParser{
		cfg: cfgPodLevelDisabled,
		l:   log.Logger().Named("test"),
	}
	ctx := context.Background()
	err := p.Start(ctx)
	require.NoError(t, err)
}

func TestInitPodLevelDisabled(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	p := &packetParser{
		cfg: cfgPodLevelDisabled,
		l:   log.Logger().Named("test"),
	}
	err := p.Init()
	require.NoError(t, err)
}

func TestPacketParseGenerate(t *testing.T) {
	takeBackup()
	defer restoreBackup()

	log.SetupZapLogger(log.GetDefaultLogOpts())
	// Get the directory of the current test file.
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to determine test file path")
	}
	currDir := path.Dir(filename)
	dynamicHeaderPath := fmt.Sprintf("%s/%s/%s", currDir, bpfSourceDir, dynamicHeaderFileName)

	// Instantiate the packetParser struct with a mocked logger and context.
	p := &packetParser{
		cfg: cfgPodLevelEnabled,
		l:   log.Logger().Named(string(Name)),
	}
	ctx := context.Background()

	// Call the Generate function and check if it returns an error.
	if err := p.Generate(ctx); err != nil {
		t.Fatalf("failed to generate PacketParser header: %v", err)
	}

	// Verify that the dynamic header file was created in the expected location and contains the expected contents.
	if _, err := os.Stat(dynamicHeaderPath); os.IsNotExist(err) {
		t.Fatalf("dynamic header file does not exist: %v", err)
	}

	expectedContents := "#define BYPASS_LOOKUP_IP_OF_INTEREST 1 \n"
	actualContents, err := os.ReadFile(dynamicHeaderPath)
	if err != nil {
		t.Fatalf("failed to read dynamic header file: %v", err)
	}
	if string(actualContents) != expectedContents {
		t.Errorf("unexpected dynamic header file contents: got %q, want %q", string(actualContents), expectedContents)
	}
}

func TestCompile(t *testing.T) {
	takeBackup()
	defer restoreBackup()

	log.SetupZapLogger(log.GetDefaultLogOpts())
	p := &packetParser{
		cfg: cfgPodLevelEnabled,
		l:   log.Logger().Named(string(Name)),
	}
	dir, _ := absPath()
	expectedOutputFile := fmt.Sprintf("%s/%s", dir, bpfObjectFileName)

	err := os.Remove(expectedOutputFile)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Expected no error. Error: %+v", err)
	}

	err = p.Generate(context.Background())
	if err != nil {
		t.Fatalf("Expected no error. Error: %+v", err)
	}

	err = p.Compile(context.Background())
	if err != nil {
		t.Fatalf("Expected no error. Error: %+v", err)
	}
	if _, err := os.Stat(expectedOutputFile); errors.Is(err, os.ErrNotExist) {
		t.Fatalf("File %+v doesn't exist", expectedOutputFile)
	}
}

// Helpers.
func takeBackup() {
	// Get the directory of the current test file.
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("failed to determine test file path")
	}
	currDir := path.Dir(filename)
	dynamicHeaderPath := fmt.Sprintf("%s/%s/%s", currDir, bpfSourceDir, dynamicHeaderFileName)

	// Rename the dynamic header file if it already exists.
	if _, err := os.Stat(dynamicHeaderPath); err == nil {
		if err := os.Rename(dynamicHeaderPath, fmt.Sprintf("%s.bak", dynamicHeaderPath)); err != nil {
			panic(fmt.Sprintf("failed to rename existing dynamic header file: %v", err))
		}
	}
}

func restoreBackup() {
	// Get the directory of the current test file.
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("failed to determine test file path")
	}
	currDir := path.Dir(filename)
	dynamicHeaderPath := fmt.Sprintf("%s/%s/%s", currDir, bpfSourceDir, dynamicHeaderFileName)

	// Remove the dynamic header file generated during test.
	os.RemoveAll(dynamicHeaderPath)

	// Restore the dynamic header file if it was renamed.
	if _, err := os.Stat(fmt.Sprintf("%s.bak", dynamicHeaderPath)); err == nil {
		if err := os.Rename(fmt.Sprintf("%s.bak", dynamicHeaderPath), dynamicHeaderPath); err != nil {
			panic(fmt.Sprintf("failed to restore dynamic header file: %v", err))
		}
	}
}
