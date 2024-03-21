// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package dropreason

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"runtime"
	"testing"
	"time"

	"github.com/cilium/ebpf/perf"
	"github.com/golang/mock/gomock"
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/metrics"
	mocks "github.com/microsoft/retina/pkg/plugin/dropreason/mocks"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

var (
	cfgPodLevelEnabled = &kcfg.Config{
		MetricsInterval:          1 * time.Second,
		EnablePodLevel:           true,
		BypassLookupIPOfInterest: true,
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
		l:   log.Logger().Named(string(Name)),
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
		l: log.Logger().Named(string(Name)),
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
		l:   log.Logger().Named(string(Name)),
	}
	dir, _ := absPath()
	expectedOutputFile := fmt.Sprintf("%s/%s", dir, bpfObjectFileName)

	err := os.Remove(expectedOutputFile)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
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

func TestProcessMapValue(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	metrics.InitializeMetrics()
	dr := &dropReason{
		cfg: cfgPodLevelEnabled,
		l:   log.Logger().Named(string(Name)),
	}

	testMetricKey := dropMetricKey{DropType: 1, ReturnVal: 2}
	testMetricValues := dropMetricValues{{Count: 10, Bytes: 100}}

	dr.processMapValue(testMetricKey, testMetricValues)

	// check if the metrics are updated
	reason := testMetricKey.getType()
	direction := testMetricKey.getDirection()

	dropCount := &dto.Metric{}
	err := metrics.DropCounter.WithLabelValues(reason, direction).Write(dropCount)
	require.Nil(t, err, "Expected no error but got: %w", err)

	dropBytes := &dto.Metric{}
	err = metrics.DropBytesCounter.WithLabelValues(reason, direction).Write(dropBytes)
	require.Nil(t, err, "Expected no error but got: %w", err)

	dropCountValue := *dropCount.Gauge.Value
	dropBytesValue := *dropBytes.Gauge.Value

	require.Equal(t, float64(testMetricValues[0].Count), dropCountValue, "Expected drop count to be %d but got %d", float64(testMetricValues[0].Count), dropCountValue)
	require.Equal(t, float64(testMetricValues[0].Bytes), dropBytesValue, "Expected drop bytes to be %d but got %d", float64(testMetricValues[0].Bytes), dropBytesValue)
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
		l:              log.Logger().Named(string(Name)),
		metricsMapData: mockedMap,
	}

	// create a ticker with a short interval for testing purposes
	ticker := time.NewTicker(1 * time.Second)

	// Create a context with a short timeout for testing purposes
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)

	// Start the drop reason routine in a goroutine
	go func() {
		if err := dr.run(ctx); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}()

	// Wait for a short period of time for the routine to start
	time.Sleep(2 * time.Second)

	cancel()
	ticker.Stop()
}

func TestDropReasonRun(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockedMap := mocks.NewMockIMap(ctrl)
	mockedMapIterator := mocks.NewMockIMapIterator(ctrl)
	mockedPerfReader := mocks.NewMockIPerfReader(ctrl)

	// reasign helper function so that it returns the mockedMapIterator
	iMapIterator = func(x IMap) IMapIterator {
		return mockedMapIterator
	}
	mockedMapIterator.EXPECT().Err().Return(nil).MinTimes(1)
	mockedMapIterator.EXPECT().Next(gomock.Any(), gomock.Any()).Return(false).MinTimes(1)

	mockedPerfRecord := perf.Record{
		CPU:         0,
		RawSample:   []byte{0x01, 0x02, 0x03},
		LostSamples: 0,
	}
	mockedPerfReader.EXPECT().Read().Return(mockedPerfRecord, nil).MinTimes(1)

	// Create drop reason instance
	dr := &dropReason{
		cfg:            cfgPodLevelEnabled,
		l:              log.Logger().Named(string(Name)),
		metricsMapData: mockedMap,
		reader:         mockedPerfReader,
		recordsChannel: make(chan perf.Record, buffer),
	}

	// Create a context with a short timeout for testing purposes
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)

	// create a ticker with a short interval for testing purposes
	ticker := time.NewTicker(2 * time.Second)

	// Start the drop reason routine in a goroutine
	go func() {
		if err := dr.run(ctx); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}()

	// Wait for a short period of time for the routine to start
	time.Sleep(2 * time.Second)

	cancel()
	ticker.Stop()
}

func TestDropReasonReadDataPodLevelEnabled(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockedMap := mocks.NewMockIMap(ctrl)
	mockedPerfReader := mocks.NewMockIPerfReader(ctrl)

	// mock perf reader record
	mockedPerfRecord := perf.Record{
		CPU:         0,
		RawSample:   []byte{0x01, 0x02, 0x03},
		LostSamples: 0,
	}
	mockedPerfReader.EXPECT().Read().Return(mockedPerfRecord, nil).MinTimes(1)

	// Create drop reason instance
	dr := &dropReason{
		cfg:            cfgPodLevelEnabled,
		l:              log.Logger().Named(string(Name)),
		metricsMapData: mockedMap,
		reader:         mockedPerfReader,
		recordsChannel: make(chan perf.Record, buffer),
	}

	// Create a context with a short timeout for testing purposes
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Start the drop reason routine in a goroutine
	go func() {
		dr.readEventArrayData()
	}()

	go func() {
		dr.wg.Add(1)
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
		l:              log.Logger().Named(string(Name)),
		metricsMapData: mockedMap,
		reader:         mockedPerfReader,
	}

	// Create a context with a short timeout for testing purposes
	_, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Start the drop reason routine in a goroutine
	go func() {
		err := dr.readEventArrayData()
		assert.Nil(t, err, "Expected error but got nil")
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

	// mock perf reader record
	mockedPerfRecord := perf.Record{
		CPU:         0,
		RawSample:   []byte{0x01, 0x02, 0x03},
		LostSamples: 3,
	}
	mockedPerfReader.EXPECT().Read().Return(mockedPerfRecord, nil).MinTimes(1)

	// Create drop reason instance
	dr := &dropReason{
		cfg:            cfgPodLevelEnabled,
		l:              log.Logger().Named(string(Name)),
		metricsMapData: mockedMap,
		reader:         mockedPerfReader,
	}

	metrics.InitializeMetrics()

	// Create a  with a short timeout for testing purposes
	_, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Start the drop reason routine in a goroutine
	go func() {
		err := dr.readEventArrayData()
		assert.Nil(t, err, "Expected error but got nil")
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

	// mock perf reader record
	mockedPerfRecord := perf.Record{
		CPU:         0,
		RawSample:   []byte{0x01, 0x02, 0x03},
		LostSamples: 3,
	}
	mockedPerfReader.EXPECT().Read().Return(mockedPerfRecord, errors.New("Unknown Error")).MinTimes(1)

	// Create drop reason instance
	dr := &dropReason{
		cfg:            cfgPodLevelEnabled,
		l:              log.Logger().Named(string(Name)),
		metricsMapData: mockedMap,
		reader:         mockedPerfReader,
	}

	// Create a context with a short timeout for testing purposes
	_, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	metrics.InitializeMetrics()

	// Start the drop reason routine in a goroutine
	go func() {
		err := dr.readEventArrayData()
		assert.NotNil(t, err, "Expected error but got nil")
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
		l:   log.Logger().Named(string(Name)),
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
