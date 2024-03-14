// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build unit
// +build unit

package packetforward

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	kcfg "github.com/microsoft/retina/pkg/config"

	"github.com/golang/mock/gomock"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/metrics"
	mocks "github.com/microsoft/retina/pkg/plugin/packetforward/mocks"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

var (
	cfgPodLevelEnabled = &kcfg.Config{
		MetricsInterval: 1 * time.Second,
		EnablePodLevel:  true,
	}
	cfgPodLevelDisabled = &kcfg.Config{
		MetricsInterval: 1 * time.Second,
		EnablePodLevel:  false,
	}
)

func TestProcessMapValue_NoErr(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	m := mocks.NewMockIMap(ctrl)
	m.EXPECT().Lookup(uint32(0), gomock.Any()).Return(nil)
	_, _, err := processMapValue(m, 0)
	if err != nil {
		t.Fatalf("Did not expect error %v", err)
	}
}

func TestProcessMapValue_ReturnErr(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	m := mocks.NewMockIMap(ctrl)
	m.EXPECT().Lookup(uint32(0), gomock.Any()).Return(errors.New("Error"))
	_, _, err := processMapValue(m, 0)
	if err == nil {
		t.Fatalf("Expected function to return error")
	}
}

func TestProcessMapValue_ReturnData(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	m := mocks.NewMockIMap(ctrl)

	v2 := []packetforwardMetric{
		{Count: 3, Bytes: 10},
		{Count: 7, Bytes: 5},
	}
	m.EXPECT().Lookup(uint32(0), gomock.Any()).SetArg(1, v2).Return(nil)
	totalCount, totalBytes, _ := processMapValue(m, 0)

	if totalCount != uint64(10) {
		t.Fatalf("Expected 10, got %v", totalCount)
	}
	if totalBytes != uint64(15) {
		t.Fatalf("Expected 15, got %v", totalBytes)
	}
}

func TestStop(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	p := &packetForward{
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

func TestStop_NonNilMap(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockedMap := mocks.NewMockIMap(ctrl)
	mockedMap.EXPECT().Close().Return(errors.New("Error")).MinTimes(1)
	p := &packetForward{
		l:           log.Logger().Named(string(Name)),
		hashmapData: mockedMap,
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

func TestCompile(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	p := &packetForward{
		l: log.Logger().Named(string(Name)),
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

func TestShutdown(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())

	p := &packetForward{
		cfg: &kcfg.Config{
			MetricsInterval: 100 * time.Second,
			EnablePodLevel:  true,
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

func TestRun(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	metrics.InitializeMetrics()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockedMap := mocks.NewMockIMap(ctrl)
	mockedMap.EXPECT().Lookup(gomock.Any(), gomock.Any()).Return(nil).MinTimes(1)

	p := &packetForward{
		cfg:         cfgPodLevelEnabled,
		l:           log.Logger().Named(string(Name)),
		hashmapData: mockedMap,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	g, errctx := errgroup.WithContext(ctx)

	// Create a ticker with a short interval for testing purpose
	ticker := time.NewTicker(1 * time.Second)

	g.Go(func() error {
		return p.run(errctx)
	})

	time.Sleep(2 * time.Second)
	cancel()
	ticker.Stop()

	err := g.Wait()
	require.NoError(t, err)
}

func TestRun_ReturnError_Ingress(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	metrics.InitializeMetrics()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockedMap := mocks.NewMockIMap(ctrl)
	mockedMap.EXPECT().Lookup(uint32(0), gomock.Any()).Return(errors.New("Error")).MinTimes(1)

	p := &packetForward{
		cfg:         cfgPodLevelEnabled,
		l:           log.Logger().Named(string(Name)),
		hashmapData: mockedMap,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	g, errctx := errgroup.WithContext(ctx)

	// Create a ticker with a short interval for testing purpose
	ticker := time.NewTicker(1 * time.Second)

	g.Go(func() error {
		return p.run(errctx)
	})

	time.Sleep(2 * time.Second)
	cancel()
	ticker.Stop()

	err := g.Wait()
	require.NoError(t, err)
}

func TestRun_ReturnError_Egress(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	metrics.InitializeMetrics()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockedMap := mocks.NewMockIMap(ctrl)
	mockedMap.EXPECT().Lookup(uint32(0), gomock.Any()).Return(nil).MinTimes(1)
	mockedMap.EXPECT().Lookup(uint32(1), gomock.Any()).Return(errors.New("Error")).MinTimes(1)

	p := &packetForward{
		cfg:         cfgPodLevelEnabled,
		l:           log.Logger().Named(string(Name)),
		hashmapData: mockedMap,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	g, errctx := errgroup.WithContext(ctx)

	// Create a ticker with a short interval for testing purpose
	ticker := time.NewTicker(1 * time.Second)

	g.Go(func() error {
		return p.run(errctx)
	})

	time.Sleep(2 * time.Second)
	cancel()
	ticker.Stop()

	err := g.Wait()
	require.NoError(t, err)
}
