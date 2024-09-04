// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package controllermanager

import (
	"context"
	"errors"
	"testing"
	"time"

	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/log"
	pm "github.com/microsoft/retina/pkg/managers/pluginmanager"
	"github.com/microsoft/retina/pkg/plugin/api"
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
	c, err := kcfg.GetConfig(testCfgFile)
	assert.NoError(t, err, "Expected no error, instead got %+v", err)
	assert.NotNil(t, c)

	log.SetupZapLogger(log.GetDefaultLogOpts())
	kubeclient := k8sfake.NewSimpleClientset()
	cm, err := NewControllerManager(c, kubeclient, telemetry.NewNoopTelemetry())
	assert.NoError(t, err, "Expected no error, instead got %+v", err)
	assert.NotNil(t, cm)
}

func TestNewControllerManagerWin(t *testing.T) {
	c, err := kcfg.GetConfig(testCfgFileWin)
	assert.NoError(t, err, "Expected no error, instead got %+v", err)
	assert.NotNil(t, c)

	log.SetupZapLogger(log.GetDefaultLogOpts())
	kubeclient := k8sfake.NewSimpleClientset()
	cm, err := NewControllerManager(c, kubeclient, telemetry.NewNoopTelemetry())
	assert.Error(t, err, "Expected error of not recognising windows plugins in linux, instead got no error")
	assert.Nil(t, cm)
}

func TestNewControllerManagerInit(t *testing.T) {
	c, err := kcfg.GetConfig(testMockCfgFile)
	assert.NoError(t, err, "Expected no error, instead got %+v", err)
	assert.NotNil(t, c)

	log.SetupZapLogger(log.GetDefaultLogOpts())
	kubeclient := k8sfake.NewSimpleClientset()
	cm, err := NewControllerManager(c, kubeclient, telemetry.NewNoopTelemetry())
	assert.NoError(t, err, "Expected no error, instead got %+v", err)
	assert.NotNil(t, cm)

	err = cm.Init(context.Background())
	assert.NoError(t, err, "Expected no error, instead got %+v", err)
}

func TestControllerPluginManagerStartFail(t *testing.T) {
	c, err := kcfg.GetConfig(testMockCfgFile)
	assert.NoError(t, err, "Expected no error, instead got %+v", err)
	assert.NotNil(t, c)

	log.SetupZapLogger(log.GetDefaultLogOpts())
	kubeclient := k8sfake.NewSimpleClientset()
	cm, err := NewControllerManager(c, kubeclient, telemetry.NewNoopTelemetry())
	assert.NoError(t, err, "Expected no error, instead got %+v", err)
	assert.NotNil(t, cm)

	ctl := gomock.NewController(t)
	defer ctl.Finish()
	log.SetupZapLogger(log.GetDefaultLogOpts())

	pluginName := "mockplugin"

	cfg := &kcfg.Config{
		MetricsInterval: timeInter,
		EnablePodLevel:  true,
	}
	mgr, err := pm.NewPluginManager(cfg, telemetry.NewNoopTelemetry(), api.PluginName(pluginName))
	require.NoError(t, err, "Expected no error, instead got %+v", err)

	mockPlugin := api.NewMockPlugin(ctl)
	mockPlugin.EXPECT().Generate(gomock.Any()).Return(nil).AnyTimes()
	mockPlugin.EXPECT().Compile(gomock.Any()).Return(nil).AnyTimes()
	mockPlugin.EXPECT().Stop().Return(nil).AnyTimes()
	mockPlugin.EXPECT().Init().Return(nil).AnyTimes()
	mockPlugin.EXPECT().Name().Return(pluginName).AnyTimes()
	mockPlugin.EXPECT().Start(gomock.Any()).Return(errors.New("test error")).AnyTimes()

	mgr.SetPlugin(api.PluginName(pluginName), mockPlugin)
	cm.pluginManager = mgr

	err = cm.Init(context.Background())
	require.NoError(t, err, "Expected no error, instead got %+v", err)

	require.Panics(t, func() { cm.Start(context.Background()) })
}
