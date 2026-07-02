// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package dropreason

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"reflect"
	"runtime"
	"testing"
	"time"
	"unsafe"

	"github.com/blang/semver/v4"
	"github.com/cilium/ebpf/perf"
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/metrics"
	mocks "github.com/microsoft/retina/pkg/plugin/dropreason/mocks"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"golang.org/x/sync/errgroup"
)

var (
	cfgPodLevelEnabled = &kcfg.Config{
		MetricsInterval:          1 * time.Second,
		EnablePodLevel:           true,
		BypassLookupIPOfInterest: true,
		DataAggregationLevel:     kcfg.Low,
	}
	cfgPodLevelDisabled = &kcfg.Config{
		MetricsInterval: 1 * time.Second,
		EnablePodLevel:  false,
	}
)

func TestStop(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	p := &dropReason{
		cfg: cfgPodLevelEnabled,
		l:   log.Logger().Named(name),
	}
	err := p.Stop()
	if err != nil {
		t.Fatalf("Expected no error")
	}
	if p.isRunning {
		t.Fatalf("Expected isRunning to be false")
	}

	p.isRunning = true
	err = p.Stop()
	if err != nil {
		t.Fatalf("Expected no error")
	}
	if p.isRunning {
		t.Fatalf("Expected isRunning to be false")
	}
}

func TestShutdown(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	p := &dropReason{
		cfg: &kcfg.Config{
			MetricsInterval: 100 * time.Second,
			EnablePodLevel:  false,
		},
		l: log.Logger().Named(name),
	}

	ctx, cancel := context.WithCancel(context.Background())
	g, errctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return p.Start(errctx)
	})

	time.Sleep(1 * time.Second)
	cancel()
	err := g.Wait()
	require.NoError(t, err)
}

func TestCompile(t *testing.T) {
	takeBackup()
	defer restoreBackup()

	log.SetupZapLogger(log.GetDefaultLogOpts())
	p := &dropReason{
		cfg: cfgPodLevelEnabled,
		l:   log.Logger().Named(name),
	}
	dir, _ := absPath()
	expectedOutputFile := fmt.Sprintf("%s/%s", dir, bpfObjectFileName)

	err := os.Remove(expectedOutputFile)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Expected no error. Error: %+v", err)
	}

	err = p.Generate(context.Background())
	if err != nil {
		t.Fatal("Expected no error. Error:", err)
	}

	err = p.Compile(context.Background())
	if err != nil {
		t.Fatalf("Expected no error. Error: %+v", err)
	}
	if _, err := os.Stat(expectedOutputFile); errors.Is(err, os.ErrNotExist) {
		t.Fatalf("File %+v doesn't exist", expectedOutputFile)
	}
}

func TestProcessMapValue(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	metrics.InitializeMetrics(slog.Default())
	dr := &dropReason{
		cfg: cfgPodLevelEnabled,
		l:   log.Logger().Named(name),
	}

	testMetricKey := dropMetricKey{DropType: 1, ReturnVal: 2}
	testMetricValues := dropMetricValues{{Count: 10, Bytes: 100}}

	dr.processMapValue(testMetricKey, testMetricValues)

	// check if the metrics are updated
	reason := testMetricKey.getType()
	direction := testMetricKey.getDirection()

	dropCount := &dto.Metric{}
	err := metrics.DropPacketsGauge.WithLabelValues(reason, direction).Write(dropCount)
	require.Nil(t, err, "Expected no error but got: %w", err)

	dropBytes := &dto.Metric{}
	err = metrics.DropBytesGauge.WithLabelValues(reason, direction).Write(dropBytes)
	require.Nil(t, err, "Expected no error but got: %w", err)

	dropCountValue := *dropCount.Gauge.Value
	dropBytesValue := *dropBytes.Gauge.Value

	require.Equal(t, float64(testMetricValues[0].Count), dropCountValue, "Expected drop count to be %d but got %d", float64(testMetricValues[0].Count), dropCountValue)
	require.Equal(t, float64(testMetricValues[0].Bytes), dropBytesValue, "Expected drop bytes to be %d but got %d", float64(testMetricValues[0].Bytes), dropBytesValue)
}

// TestProcessMapValue_TCPAcceptBasicWithError verifies that TCP_ACCEPT_BASIC
// entries with a real error code (not EAGAIN) are correctly reported.
// After the fix, the eBPF program filters out EAGAIN (-11) and only writes
// genuine errors to the map with their error code in ReturnVal.
func TestProcessMapValue_TCPAcceptBasicWithError(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	metrics.InitializeMetrics(slog.Default())
	dr := &dropReason{
		cfg: cfgPodLevelEnabled,
		l:   log.Logger().Named(name),
	}

	// TCP_ACCEPT_BASIC = 3, with a real error like -ENOMEM (-12).
	testMetricKey := dropMetricKey{DropType: 3, ReturnVal: -12}
	testMetricValues := dropMetricValues{{Count: 5, Bytes: 0}}

	dr.processMapValue(testMetricKey, testMetricValues)

	reason := testMetricKey.getType()
	direction := testMetricKey.getDirection()
	require.Equal(t, "TCP_ACCEPT_BASIC", reason)
	require.Equal(t, "ingress", direction)

	dropCount := &dto.Metric{}
	err := metrics.DropPacketsGauge.WithLabelValues(reason, direction).Write(dropCount)
	require.NoError(t, err)
	require.Equal(t, float64(5), *dropCount.Gauge.Value)
}

// TestProcessMapValue_TCPAcceptBasicEAGAINNotInMap documents that after the
// eBPF fix, EAGAIN errors are filtered in-kernel and never appear in the map.
// This test verifies that if somehow an EAGAIN entry did appear (e.g. race
// during upgrade), it would still be processed — the filtering is in eBPF.
func TestProcessMapValue_TCPAcceptBasicEAGAINNotInMap(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	metrics.InitializeMetrics(slog.Default())
	dr := &dropReason{
		cfg: cfgPodLevelEnabled,
		l:   log.Logger().Named(name),
	}

	// Simulate what the OLD buggy code would produce: TCP_ACCEPT_BASIC with
	// ReturnVal=0 (the old fexit didn't pass any error code).
	testMetricKey := dropMetricKey{DropType: 3, ReturnVal: 0}
	testMetricValues := dropMetricValues{{Count: 942303, Bytes: 0}}

	dr.processMapValue(testMetricKey, testMetricValues)

	reason := testMetricKey.getType()
	direction := testMetricKey.getDirection()
	require.Equal(t, "TCP_ACCEPT_BASIC", reason)
	require.Equal(t, "ingress", direction)

	// The Go side still processes whatever the map contains; the fix is that
	// the eBPF program no longer writes these entries for EAGAIN.
	dropCount := &dto.Metric{}
	err := metrics.DropPacketsGauge.WithLabelValues(reason, direction).Write(dropCount)
	require.NoError(t, err)
	require.Equal(t, float64(942303), *dropCount.Gauge.Value)
}

// TestDropMetricKey_GetDirection verifies direction mapping for all drop types.
func TestDropMetricKey_GetDirection(t *testing.T) {
	tests := []struct {
		dropType  uint16
		wantDir   string
		wantType  string
	}{
		{dropType: 0, wantDir: "unknown", wantType: "IPTABLE_RULE_DROP"},
		{dropType: 1, wantDir: "unknown", wantType: "IPTABLE_NAT_DROP"},
		{dropType: 2, wantDir: "egress", wantType: "TCP_CONNECT_BASIC"},
		{dropType: 3, wantDir: "ingress", wantType: "TCP_ACCEPT_BASIC"},
		{dropType: 5, wantDir: "unknown", wantType: "CONNTRACK_ADD_DROP"},
	}
	for _, tt := range tests {
		t.Run(tt.wantType, func(t *testing.T) {
			dk := &dropMetricKey{DropType: tt.dropType}
			require.Equal(t, tt.wantDir, dk.getDirection())
			require.Equal(t, tt.wantType, dk.getType())
		})
	}
}

func TestDropReasonRun_Error(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockedMap := mocks.NewMockIMap(ctrl)
	mockedMapIterator := mocks.NewMockIMapIterator(ctrl)

	// reasign helper function so that it returns the mockedMapIterator
	iMapIterator = func(x IMap) IMapIterator {
		return mockedMapIterator
	}
	mockedMapIterator.EXPECT().Err().Return(errors.New("test error")).MinTimes(1)
	mockedMapIterator.EXPECT().Next(gomock.Any(), gomock.Any()).Return(false).MinTimes(1)

	// Create drop reason instance
	dr := &dropReason{
		cfg:            cfgPodLevelDisabled,
		l:              log.Logger().Named(name),
		metricsMapData: mockedMap,
	}

	// create a ticker with a short interval for testing purposes
	ticker := time.NewTicker(1 * time.Second)

	// Create a context with a short timeout for testing purposes
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)

	errCh := make(chan error, 1)
	// Start the drop reason routine in a goroutine
	go func() {
		errCh <- dr.run(ctx)
	}()

	// Wait for a short period of time for the routine to start
	time.Sleep(2 * time.Second)

	cancel()
	ticker.Stop()
	if err := <-errCh; err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDropReasonRun(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockedMap := mocks.NewMockIMap(ctrl)
	mockedMapIterator := mocks.NewMockIMapIterator(ctrl)
	mockedPerfReader := mocks.NewMockIPerfReader(ctrl)
	menricher := enricher.NewMockEnricherInterface(ctrl) //nolint:typecheck

	// reasign helper function so that it returns the mockedMapIterator
	iMapIterator = func(x IMap) IMapIterator {
		return mockedMapIterator
	}
	mockedMapIterator.EXPECT().Err().Return(nil).MinTimes(1)
	mockedMapIterator.EXPECT().Next(gomock.Any(), gomock.Any()).Return(false).MinTimes(1)

	// create a rawSample slice and fill it with `unsafe.Sizeof(kprobePacket{})`
	rawSample := make([]byte, unsafe.Sizeof(kprobePacket{}))
	for i := range rawSample {
		rawSample[i] = byte(i)
	}

	// TODO(nddq) : test an actual kprobePacket similar to what we are doing with packetparserPacket in packetparser
	mockedPerfRecord := perf.Record{
		CPU:         0,
		RawSample:   rawSample,
		LostSamples: 0,
	}
	mockedPerfReader.EXPECT().Read().Return(mockedPerfRecord, nil).MinTimes(1)

	// Create drop reason instance
	dr := &dropReason{
		cfg:            cfgPodLevelEnabled,
		l:              log.Logger().Named(name),
		metricsMapData: mockedMap,
		reader:         mockedPerfReader,
		enricher:       menricher,
		recordsChannel: make(chan perf.Record, buffer),
	}
	menricher.EXPECT().Write(gomock.Any()).MinTimes(1)

	// Create a context with a short timeout for testing purposes
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)

	// create a ticker with a short interval for testing purposes
	ticker := time.NewTicker(2 * time.Second)

	errCh := make(chan error, 1)
	// Start the drop reason routine in a goroutine
	go func() {
		errCh <- dr.run(ctx)
	}()

	// Wait for a short period of time for the routine to start
	time.Sleep(2 * time.Second)

	cancel()
	ticker.Stop()
	if err := <-errCh; err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDropReasonReadDataPodLevelEnabled(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockedMap := mocks.NewMockIMap(ctrl)
	mockedPerfReader := mocks.NewMockIPerfReader(ctrl)
	menricher := enricher.NewMockEnricherInterface(ctrl) //nolint:typecheck

	// create a rawSample slice and fill it with `unsafe.Sizeof(kprobePacket{})`
	rawSample := make([]byte, unsafe.Sizeof(kprobePacket{}))
	for i := range rawSample {
		rawSample[i] = byte(i)
	}

	mockedPerfRecord := perf.Record{
		CPU:         0,
		RawSample:   rawSample,
		LostSamples: 0,
	}

	mockedPerfReader.EXPECT().Read().Return(mockedPerfRecord, nil).MinTimes(1)
	menricher.EXPECT().Write(gomock.Any()).MinTimes(1)

	// Create drop reason instance
	dr := &dropReason{
		cfg:            cfgPodLevelEnabled,
		l:              log.Logger().Named(name),
		metricsMapData: mockedMap,
		reader:         mockedPerfReader,
		enricher:       menricher,
		recordsChannel: make(chan perf.Record, buffer),
	}

	// Create a context with a short timeout for testing purposes
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Start the drop reason routine in a goroutine
	go func() {
		dr.readEventArrayData()
	}()

	dr.wg.Add(1)
	go func() {
		dr.processRecord(ctx, 0)
	}()

	// Wait for a short period of time for the routine to start
	// time.Sleep(2 * time.Second)
	<-ctx.Done()
}

func TestDropReasonReadData_WithEmptyPerfArray(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockedMap := mocks.NewMockIMap(ctrl)
	mockedPerfReader := mocks.NewMockIPerfReader(ctrl)

	// mock perf reader record
	mockedPerfRecord := perf.Record{
		CPU:         0,
		RawSample:   []byte{},
		LostSamples: 0,
	}
	mockedPerfReader.EXPECT().Read().Return(mockedPerfRecord, perf.ErrClosed).MinTimes(1)

	// Create drop reason instance
	dr := &dropReason{
		cfg:            cfgPodLevelEnabled,
		l:              log.Logger().Named(name),
		metricsMapData: mockedMap,
		reader:         mockedPerfReader,
	}

	// Create a context with a short timeout for testing purposes
	_, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Start the drop reason routine in a goroutine
	go func() {
		dr.readEventArrayData()
	}()

	// Wait for a short period of time for the routine to start
	time.Sleep(2 * time.Second)
}

func TestDropReasonReadData_WithPerfArrayLostSamples(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockedMap := mocks.NewMockIMap(ctrl)
	mockedPerfReader := mocks.NewMockIPerfReader(ctrl)

	// create a rawSample slice and fill it with `unsafe.Sizeof(kprobePacket{})`
	rawSample := make([]byte, unsafe.Sizeof(kprobePacket{}))
	for i := range rawSample {
		rawSample[i] = byte(i)
	}

	mockedPerfRecord := perf.Record{
		CPU:         0,
		RawSample:   rawSample,
		LostSamples: 3,
	}
	mockedPerfReader.EXPECT().Read().Return(mockedPerfRecord, nil).MinTimes(1)

	// Create drop reason instance
	dr := &dropReason{
		cfg:            cfgPodLevelEnabled,
		l:              log.Logger().Named(name),
		metricsMapData: mockedMap,
		reader:         mockedPerfReader,
	}

	// Create a context with a short timeout for testing purposes
	_, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Start the drop reason routine in a goroutine
	go func() {
		dr.readEventArrayData()
	}()

	// Wait for a short period of time for the routine to start
	time.Sleep(2 * time.Second)
}

func TestDropReasonReadData_WithUnknownError(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockedMap := mocks.NewMockIMap(ctrl)
	mockedPerfReader := mocks.NewMockIPerfReader(ctrl)

	// create a rawSample slice and fill it with `unsafe.Sizeof(kprobePacket{})`
	rawSample := make([]byte, unsafe.Sizeof(kprobePacket{}))
	for i := range rawSample {
		rawSample[i] = byte(i)
	}

	mockedPerfRecord := perf.Record{
		CPU:         0,
		RawSample:   rawSample,
		LostSamples: 3,
	}
	mockedPerfReader.EXPECT().Read().Return(mockedPerfRecord, errors.New("Unknown Error")).MinTimes(1)

	// Create drop reason instance
	dr := &dropReason{
		cfg:            cfgPodLevelEnabled,
		l:              log.Logger().Named(name),
		metricsMapData: mockedMap,
		reader:         mockedPerfReader,
	}

	// Create a context with a short timeout for testing purposes
	_, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Start the drop reason routine in a goroutine
	go func() {
		dr.readEventArrayData()
	}()

	// Wait for a short period of time for the routine to start
	time.Sleep(2 * time.Second)
}

func TestDropReasonGenerate(t *testing.T) {
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

	// Instantiate the dropReason struct with a mocked logger and context.
	dr := &dropReason{
		cfg: cfgPodLevelEnabled,
		l:   log.Logger().Named(name),
	}
	ctx := context.Background()

	// Call the Generate function and check if it returns an error.
	if err := dr.Generate(ctx); err != nil {
		t.Fatalf("failed to generate DropReason header: %v", err)
	}

	// Verify that the dynamic header file was created in the expected location and contains the expected contents.
	if _, err := os.Stat(dynamicHeaderPath); os.IsNotExist(err) {
		t.Fatalf("dynamic header file does not exist: %v", err)
	}

	expectedContents := "#define ADVANCED_METRICS 1 \n#define BYPASS_LOOKUP_IP_OF_INTEREST 1 \n"
	actualContents, err := os.ReadFile(dynamicHeaderPath)
	if err != nil {
		t.Fatalf("failed to read dynamic header file: %v", err)
	}
	if string(actualContents) != expectedContents {
		t.Errorf("unexpected dynamic header file contents: got %q, want %q", string(actualContents), expectedContents)
	}
}

func mustVersion(v string) semver.Version {
	ver, err := semver.Parse(v)
	if err != nil {
		panic(err)
	}
	return ver
}

func TestResolveEbpfPayload(t *testing.T) {
	tests := []struct {
		name              string
		arch              string
		kv                semver.Version
		isMariner         bool
		isPodLevel        bool
		ftraceEnabled     bool
		wantType          string
		wantSupportsFexit bool
	}{
		{
			name:              "old kernel - fallback to allKprobeObjects",
			arch:              "amd64",
			kv:                mustVersion("5.4.0"),
			isMariner:         false,
			isPodLevel:        false,
			ftraceEnabled:     true,
			wantType:          "*dropreason.allKprobeObjects",
			wantSupportsFexit: false,
		},
		{
			name:              "new kernel - fexitObjects for Ubuntu",
			arch:              "amd64",
			kv:                mustVersion("5.10.0"),
			isMariner:         false,
			isPodLevel:        false,
			ftraceEnabled:     true,
			wantType:          "*dropreason.allFexitObjects",
			wantSupportsFexit: true,
		},
		{
			name:              "new kernel - marinerObjects for Mariner",
			arch:              "amd64",
			kv:                mustVersion("5.10.0"),
			isMariner:         true,
			isPodLevel:        false,
			ftraceEnabled:     true,
			wantType:          "*dropreason.marinerObjects",
			wantSupportsFexit: true,
		},
		{
			name:              "arm64 old kernel - fallback to allKprobeObjects",
			arch:              "arm64",
			kv:                mustVersion("5.8.0"),
			isMariner:         true,
			isPodLevel:        false,
			ftraceEnabled:     true,
			wantType:          "*dropreason.allKprobeObjects",
			wantSupportsFexit: false,
		},
		{
			name:              "arm64 new kernel - marinerObjects",
			arch:              "arm64",
			kv:                mustVersion("6.1.0"),
			isMariner:         true,
			isPodLevel:        false,
			ftraceEnabled:     true,
			wantType:          "*dropreason.marinerObjects",
			wantSupportsFexit: true,
		},
		{
			name:              "pod level - use allKprobeObjects",
			arch:              "amd64",
			kv:                mustVersion("5.15.0"),
			isMariner:         false,
			isPodLevel:        true,
			ftraceEnabled:     true,
			wantType:          "*dropreason.allKprobeObjects",
			wantSupportsFexit: false,
		},
		{
			name:              "mariner with ftrace disabled - fallback to kprobes",
			arch:              "amd64",
			kv:                mustVersion("5.15.0"),
			isMariner:         true,
			isPodLevel:        false,
			ftraceEnabled:     false,
			wantType:          "*dropreason.allKprobeObjects",
			wantSupportsFexit: false,
		},
		{
			name:              "ubuntu with ftrace disabled - fallback to kprobes",
			arch:              "amd64",
			kv:                mustVersion("6.6.0"),
			isMariner:         false,
			isPodLevel:        false,
			ftraceEnabled:     false,
			wantType:          "*dropreason.allKprobeObjects",
			wantSupportsFexit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objs, _, isFexit := resolvePayload(tt.arch, tt.kv, tt.isMariner, tt.isPodLevel, tt.ftraceEnabled)

			if isFexit != tt.wantSupportsFexit {
				t.Errorf("isFexit = %v, want %v", isFexit, tt.wantSupportsFexit)
			}

			if gotType := reflect.TypeOf(objs).String(); gotType != tt.wantType {
				t.Errorf("object type = %v, want %v", gotType, tt.wantType)
			}
		})
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
