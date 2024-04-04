// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package pluginmanager

import (
	"context"
	"errors"
	"testing"
	"time"

	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/log"
	watchermock "github.com/microsoft/retina/pkg/managers/watchermanager/mocks"
	"github.com/microsoft/retina/pkg/metrics"
	"github.com/microsoft/retina/pkg/plugin/api"
	pluginmock "github.com/microsoft/retina/pkg/plugin/api/mock"
	"github.com/microsoft/retina/pkg/telemetry"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"golang.org/x/sync/errgroup"
)

const (
	timeInter = time.Second * 10
)

var (
	cfgPodLevelEnabled = &kcfg.Config{
		MetricsInterval: timeInter,
		EnablePodLevel:  true,
	}
	cfgPodLevelDisabled = &kcfg.Config{
		MetricsInterval: timeInter,
		EnablePodLevel:  false,
	}
)

func setupWatcherManagerMock(ctl *gomock.Controller) (m *watchermock.MockIWatcherManager) {
	m = watchermock.NewMockIWatcherManager(ctl)
	m.EXPECT().Start(gomock.Any()).Return(nil).AnyTimes()
	m.EXPECT().Stop(gomock.Any()).Return(nil).AnyTimes()
	return
}

func TestNewManager(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	tel := telemetry.NewNoopTelemetry()
	tests := []struct {
		name       string
		cfg        *kcfg.Config
		pluginName string
		wantErr    bool
	}{
		{
			name:       "New Manager success Pod Level Disabled",
			cfg:        cfgPodLevelDisabled,
			pluginName: "mockplugin",
			wantErr:    false,
		},
		{
			name:       "New Manager success Pod Level Enabled",
			cfg:        cfgPodLevelEnabled,
			pluginName: "mockplugin",
			wantErr:    false,
		},
		{
			name:       "Fail with plugin not found Pod Level Disabled",
			cfg:        cfgPodLevelDisabled,
			pluginName: "fakeplugin",
			wantErr:    true,
		},
		{
			name:       "Fail with plugin not found Pod Level Enabled",
			cfg:        cfgPodLevelEnabled,
			pluginName: "fakeplugin",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		mgr, err := NewPluginManager(tt.cfg, tel, api.PluginName(tt.pluginName))
		if tt.wantErr {
			require.NotNil(t, err, "Expected error but got nil")
			require.Nil(t, mgr, "Expected mgr to be nil but it isn't")
		} else {
			require.Nil(t, err, "Expected nil but got error:%w", err)
			require.NotNil(t, mgr, "Expected mgr to be intialized but found nil")
			require.Condition(t, assert.Comparison(func() bool {
				_, ok := mgr.plugins[api.PluginName(tt.pluginName)]
				return ok
			}), "plugin not found in mgr map")
		}
	}
}

func TestNewManagerStart(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	tel := telemetry.NewNoopTelemetry()
	tests := []struct {
		name       string
		cfg        *kcfg.Config
		pluginName string
		wantErr    bool
	}{
		{
			name:       "New Manager success Pod Level Disabled",
			cfg:        cfgPodLevelDisabled,
			pluginName: "mockplugin",
			wantErr:    false,
		},
		{
			name:       "New Manager success Pod Level Enabled",
			cfg:        cfgPodLevelEnabled,
			pluginName: "mockplugin",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		mgr, err := NewPluginManager(tt.cfg, tel, api.PluginName(tt.pluginName))
		require.Nil(t, err, "Expected nil but got error:%w", err)
		require.NotNil(t, mgr, "Expected mgr to be intialized but found nil")
		require.Condition(t, assert.Comparison(func() bool {
			_, ok := mgr.plugins[api.PluginName(tt.pluginName)]
			return ok
		}), "plugin not found in mgr map")

		ctx, cancel := context.WithCancel(context.Background())

		go func() {
			err = mgr.Start(ctx)
			require.Nil(t, err, "Expected nil but got error:%w", err)
		}()

		time.Sleep(1 * time.Second)
		cancel()
	}
}

func TestNewManagerWithPluginStartFailure(t *testing.T) {
	ctl := gomock.NewController(t)
	defer ctl.Finish()
	log.SetupZapLogger(log.GetDefaultLogOpts())

	pluginName := "mockplugin"

	mgr := &PluginManager{
		cfg:            cfgPodLevelEnabled,
		l:              log.Logger().Named("plugin-manager"),
		plugins:        make(map[api.PluginName]api.Plugin),
		tel:            telemetry.NewNoopTelemetry(),
		watcherManager: setupWatcherManagerMock(ctl),
	}

	mockPlugin := pluginmock.NewMockPlugin(ctl)
	mockPlugin.EXPECT().Generate(gomock.Any()).Return(nil).AnyTimes()
	mockPlugin.EXPECT().Compile(gomock.Any()).Return(nil).AnyTimes()
	mockPlugin.EXPECT().Stop().Return(nil).AnyTimes()
	mockPlugin.EXPECT().Init().Return(nil).AnyTimes()
	mockPlugin.EXPECT().Start(gomock.Any()).Return(errors.New("Plugin failed to start")).AnyTimes()
	mockPlugin.EXPECT().Name().Return(pluginName).AnyTimes()

	mgr.plugins[api.PluginName(pluginName)] = mockPlugin

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		err := mgr.Start(ctx)
		require.NotNil(t, err, "Expected Error but got nil:%w", err)
		require.ErrorContains(t, err, "Plugin failed to start", err, "Expected error to contain 'Plugin failed to start' but got %s", err.Error())
	}()

	time.Sleep(1 * time.Second)
	cancel()
}

func TestNewManagerWithPluginReconcileFailure(t *testing.T) {
	ctl := gomock.NewController(t)
	defer ctl.Finish()
	log.SetupZapLogger(log.GetDefaultLogOpts())
	metrics.InitializeMetrics()

	pluginName := "mockplugin"

	mgr := &PluginManager{
		cfg:            cfgPodLevelEnabled,
		l:              log.Logger().Named("plugin-manager"),
		plugins:        make(map[api.PluginName]api.Plugin),
		tel:            telemetry.NewNoopTelemetry(),
		watcherManager: setupWatcherManagerMock(ctl),
	}

	mockPlugin := pluginmock.NewMockPlugin(ctl)
	mockPlugin.EXPECT().Generate(gomock.Any()).Return(nil).AnyTimes()
	mockPlugin.EXPECT().Compile(gomock.Any()).Return(nil).AnyTimes()
	mockPlugin.EXPECT().Stop().Return(errors.New("Plugin failed to stop")).AnyTimes()
	mockPlugin.EXPECT().Init().Return(nil).AnyTimes()
	mockPlugin.EXPECT().Start(gomock.Any()).Return(nil).AnyTimes()
	mockPlugin.EXPECT().Name().Return(pluginName).AnyTimes()

	mgr.plugins[api.PluginName(pluginName)] = mockPlugin

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		err := mgr.Start(ctx)
		require.NotNil(t, err, "Expected Error but got nil:%w", err)
		require.ErrorContains(t, err, "Plugin failed to stop", err, "Expected error to contain 'Plugin failed to start' but got %s", err.Error())
		count, error := metrics.PluginManagerFailedToReconcileCounter.GetMetricWithLabelValues(pluginName)
		out := &dto.Metric{}
		count.Write(out)
		require.Nil(t, error, "Expected nil but got error:%w", error)
		require.Equal(t, float64(1), *out.Counter.Value, "Expected 1 but got %f", *out.Counter.Value)
	}()

	time.Sleep(1 * time.Second)
	cancel()
}

func TestPluginInit(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	tel := telemetry.NewNoopTelemetry()
	tests := []struct {
		name       string
		cfg        *kcfg.Config
		pluginName string
		wantErr    bool
	}{
		{
			name:       "Plugin init successes Pod Level Disabled",
			cfg:        cfgPodLevelDisabled,
			pluginName: "mockplugin",
			wantErr:    false,
		},
		{
			name:       "Plugin init successes Pod Level Enabled",
			cfg:        cfgPodLevelEnabled,
			pluginName: "mockplugin",
			wantErr:    false,
		},
	}
	for _, tt := range tests {
		mgr, err := NewPluginManager(tt.cfg, tel, api.PluginName(tt.pluginName))
		require.Nil(t, err, "Expected nil but got error:%w", err)
		for _, plugin := range mgr.plugins {
			if tt.wantErr {
				err := plugin.Init()
				require.NotNil(t, err, "Expected Init err but got nil")
				require.ErrorContains(t, err, "event channel is nil", "Expected event channel nil but got:%w", err)
			} else {
				err := plugin.Init()
				require.Nil(t, err, "Expected nil but got init error:%w", err)
			}
		}
	}
}

func TestPluginStartWithoutInit(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	tel := telemetry.NewNoopTelemetry()
	tests := []struct {
		name       string
		cfg        *kcfg.Config
		pluginName string
		wantErr    bool
		initPlugin bool
	}{
		{
			name:       "Plugin start successes Pod Level Disabled",
			cfg:        cfgPodLevelDisabled,
			pluginName: "mockplugin",
			wantErr:    true,
			initPlugin: false,
		},
		{
			name:       "Plugin start successes Pod Level Enabled",
			cfg:        cfgPodLevelEnabled,
			pluginName: "mockplugin",
			wantErr:    false,
			initPlugin: true,
		},
	}
	for _, tt := range tests {
		mgr, err := NewPluginManager(tt.cfg, tel, api.PluginName(tt.pluginName))
		require.Nil(t, err, "Expected nil but got error:%w", err)
		for _, plugin := range mgr.plugins {
			if tt.initPlugin {
				err := plugin.Init()
				require.Nil(t, err, "Expected nil but got init error:%w", err)
			}
			if tt.wantErr {
				err := plugin.Start(context.Background())
				require.NotNil(t, err, "Expected Start err but got nil")
				require.ErrorContains(t, err, "plugin not initialized", "Expected event channel nil but got:%w", err)
			} else {
				err = plugin.Start(context.Background())
				require.Nil(t, err, "Expected nil but got start error:%w", err)
			}
		}
	}
}

func TestPluginStop(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	tel := telemetry.NewNoopTelemetry()
	tests := []struct {
		name         string
		cfg          *kcfg.Config
		pluginName   string
		wantStartErr bool
		wantStopErr  bool
		initPlugin   bool
		startPlugin  bool
	}{
		{
			name:         "Plugin stop successes Pod Level Disabled",
			cfg:          cfgPodLevelDisabled,
			pluginName:   "mockplugin",
			wantStartErr: false,
			wantStopErr:  false,
			initPlugin:   true,
			startPlugin:  true,
		},
		{
			name:         "Plugin stop failure Pod Level Disabled",
			cfg:          cfgPodLevelDisabled,
			pluginName:   "mockplugin",
			wantStartErr: true,
			wantStopErr:  false,
			initPlugin:   false,
			startPlugin:  false,
		},
		{
			name:         "Plugin stop failure Pod Level Disabled",
			cfg:          cfgPodLevelDisabled,
			pluginName:   "mockplugin",
			wantStartErr: false,
			wantStopErr:  false,
			initPlugin:   true,
			startPlugin:  true,
		},
		{
			name:         "Plugin stop successes Pod Level Enabled",
			cfg:          cfgPodLevelEnabled,
			pluginName:   "mockplugin",
			wantStartErr: false,
			wantStopErr:  false,
			initPlugin:   true,
			startPlugin:  true,
		},
		{
			name:         "Plugin stop failure Pod Level Enabled",
			cfg:          cfgPodLevelEnabled,
			pluginName:   "mockplugin",
			wantStartErr: true,
			wantStopErr:  false,
			initPlugin:   false,
			startPlugin:  false,
		},
		{
			name:         "Plugin stop failure Pod Level Enabled",
			cfg:          cfgPodLevelEnabled,
			pluginName:   "mockplugin",
			wantStartErr: false,
			wantStopErr:  false,
			initPlugin:   true,
			startPlugin:  true,
		},
	}
	for _, tt := range tests {
		mgr, err := NewPluginManager(tt.cfg, tel, api.PluginName(tt.pluginName))
		require.Nil(t, err, "Expected nil but got error:%w", err)
		for _, plugin := range mgr.plugins {
			if tt.initPlugin {
				err := plugin.Init()
				require.Nil(t, err, "Expected nil but got init error:%w", err)
			}
			if tt.startPlugin {
				err := plugin.Start(context.Background())
				if tt.wantStartErr {
					require.NotNil(t, err, "Expected nil but got start error:%w", err)
				} else {
					require.Nil(t, err, "Expected nil but got stop error:%w", err)
				}
			}
			err := plugin.Stop()
			if tt.wantStopErr {
				require.NotNil(t, err, "Expected stop err but got nil")
			} else {
				require.Nil(t, err, "Expected nil but got stop error:%w", err)
			}
		}
	}
}

func TestStopPluginManagerGracefully(t *testing.T) {
	ctl := gomock.NewController(t)
	defer ctl.Finish()
	log.SetupZapLogger(log.GetDefaultLogOpts())

	pluginName := "mockplugin"

	mgr := &PluginManager{
		cfg:            cfgPodLevelEnabled,
		l:              log.Logger().Named("plugin-manager"),
		plugins:        make(map[api.PluginName]api.Plugin),
		tel:            telemetry.NewNoopTelemetry(),
		watcherManager: setupWatcherManagerMock(ctl),
	}

	mockPlugin := pluginmock.NewMockPlugin(ctl)
	mockPlugin.EXPECT().Generate(gomock.Any()).Return(nil).AnyTimes()
	mockPlugin.EXPECT().Compile(gomock.Any()).Return(nil).AnyTimes()
	mockPlugin.EXPECT().Stop().Return(nil).AnyTimes()
	mockPlugin.EXPECT().Init().Return(nil).AnyTimes()
	mockPlugin.EXPECT().Start(gomock.Any()).Return(nil).AnyTimes()
	mockPlugin.EXPECT().Name().Return(pluginName).AnyTimes()

	mgr.plugins[api.PluginName(pluginName)] = mockPlugin

	ctx, cancel := context.WithCancel(context.Background())
	g, errctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return mgr.Start(errctx)
	})

	time.Sleep(1 * time.Second)
	cancel()
	err := g.Wait()
	require.NoError(t, err)
}

func TestWatcherManagerFailure(t *testing.T) {
	ctl := gomock.NewController(t)
	defer ctl.Finish()
	log.SetupZapLogger(log.GetDefaultLogOpts())

	m := watchermock.NewMockIWatcherManager(ctl)
	m.EXPECT().Start(gomock.Any()).Return(errors.New("error")).AnyTimes()

	mgr := &PluginManager{
		cfg:            cfgPodLevelEnabled,
		l:              log.Logger().Named("plugin-manager"),
		plugins:        make(map[api.PluginName]api.Plugin),
		tel:            telemetry.NewNoopTelemetry(),
		watcherManager: m,
	}

	err := mgr.Start(context.Background())
	require.NotNil(t, err, "Expected Start err but got nil")
	require.ErrorContains(t, err, "failed to start watcher manager", "Expected watcher manager , but got:%w", err)
}
