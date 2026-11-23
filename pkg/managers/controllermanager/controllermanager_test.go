// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package controllermanager

import (
	"context"
	"errors"
	"log/slog"
	"runtime"
	"testing"
	"time"

	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/log"
	pm "github.com/microsoft/retina/pkg/managers/pluginmanager"
	plugin "github.com/microsoft/retina/pkg/plugin/mock"
	_ "github.com/microsoft/retina/pkg/plugin/mockplugin"
	"github.com/microsoft/retina/pkg/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

const (
	testCfgFile     = "../../config/testwith/config.yaml"
	testMockCfgFile = "../../config/testwith/config-mock.yaml"
	testCfgFileWin  = "../../config/testwith/config-win.yaml"
	timeInter       = time.Second * 10
)

func TestNewControllerManager(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Linux plugin configuration is not valid on Windows")
	}

	c, err := kcfg.GetConfig(testCfgFile)
	require.NoError(t, err, "Expected no error, instead got %+v", err)
	require.NotNil(t, c)

	log.SetupZapLogger(log.GetDefaultLogOpts())
	kubeclient := k8sfake.NewSimpleClientset()
	cm, err := NewControllerManager(c, kubeclient, telemetry.NewNoopTelemetry(), slog.Default())
	require.NoError(t, err, "Expected no error, instead got %+v", err)
	require.NotNil(t, cm)
}

func TestNewControllerManagerWin(t *testing.T) {
	c, err := kcfg.GetConfig(testCfgFileWin)
	assert.NoError(t, err, "Expected no error, instead got %+v", err)
	assert.NotNil(t, c)

	log.SetupZapLogger(log.GetDefaultLogOpts())
	kubeclient := k8sfake.NewSimpleClientset()
	cm, err := NewControllerManager(c, kubeclient, telemetry.NewNoopTelemetry(), slog.Default())
	if runtime.GOOS == "windows" {
		require.NoError(t, err, "Expected Windows plugin configuration to be recognized")
		require.NotNil(t, cm)
		return
	}

	assert.Error(t, err, "Expected error of not recognising windows plugins in linux, instead got no error")
	assert.Nil(t, cm)
}

func TestNewControllerManagerInit(t *testing.T) {
	c, err := kcfg.GetConfig(testMockCfgFile)
	require.NoError(t, err, "Expected no error, instead got %+v", err)
	require.NotNil(t, c)

	log.SetupZapLogger(log.GetDefaultLogOpts())
	kubeclient := k8sfake.NewSimpleClientset()
	cm, err := NewControllerManager(c, kubeclient, telemetry.NewNoopTelemetry(), slog.Default())
	require.NoError(t, err, "Expected no error, instead got %+v", err)
	require.NotNil(t, cm)

	err = cm.Init(context.Background())
	require.NoError(t, err, "Expected no error, instead got %+v", err)
}

func TestControllerPluginManagerStartFail(t *testing.T) {
	c, err := kcfg.GetConfig(testMockCfgFile)
	assert.NoError(t, err, "Expected no error, instead got %+v", err)
	assert.NotNil(t, c)

	log.SetupZapLogger(log.GetDefaultLogOpts())
	kubeclient := k8sfake.NewSimpleClientset()
	cm, err := NewControllerManager(c, kubeclient, telemetry.NewNoopTelemetry(), slog.Default())
	assert.NoError(t, err, "Expected no error, instead got %+v", err)
	assert.NotNil(t, cm)

	ctl := gomock.NewController(t)
	defer ctl.Finish()
	log.SetupZapLogger(log.GetDefaultLogOpts())

	pluginName := "mockplugin"
	cfg := &kcfg.Config{
		MetricsInterval: timeInter,
		EnablePodLevel:  true,
		EnabledPlugin:   []string{pluginName},
	}
	mgr, err := pm.NewPluginManager(cfg, telemetry.NewNoopTelemetry(), slog.Default())
	require.NoError(t, err, "Expected no error, instead got %+v", err)

	mockPlugin := plugin.NewMockPlugin(ctl)
	mockPlugin.EXPECT().Generate(gomock.Any()).Return(nil).AnyTimes()
	mockPlugin.EXPECT().Compile(gomock.Any()).Return(nil).AnyTimes()
	mockPlugin.EXPECT().Stop().Return(nil).AnyTimes()
	mockPlugin.EXPECT().Init().Return(nil).AnyTimes()
	mockPlugin.EXPECT().Name().Return(pluginName).AnyTimes()
	mockPlugin.EXPECT().Start(gomock.Any()).Return(errors.New("test error")).AnyTimes()

	mgr.SetPlugin(pluginName, mockPlugin)
	cm.pluginManager = mgr

	err = cm.Init(context.Background())
	require.NoError(t, err, "Expected no error, instead got %+v", err)

	require.Panics(t, func() { cm.Start(context.Background()) })
}
