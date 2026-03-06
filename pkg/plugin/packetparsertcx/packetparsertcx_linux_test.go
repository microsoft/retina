// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package packetparsertcx

import (
	"context"
	"sync"
	"testing"

	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	cfgPodLevelEnabled = &kcfg.Config{
		EnablePodLevel:           true,
		BypassLookupIPOfInterest: true,
		EnableConntrackMetrics:   false,
	}
	cfgPodLevelDisabled = &kcfg.Config{
		EnablePodLevel: false,
	}
)

func TestNew(t *testing.T) {
	_, err := log.SetupZapLogger(log.GetDefaultLogOpts())
	require.NoError(t, err)

	p := New(cfgPodLevelEnabled)
	require.NotNil(t, p)
	assert.Equal(t, "packetparsertcx", p.Name())
}

func TestInitPodLevelDisabled(t *testing.T) {
	_, err := log.SetupZapLogger(log.GetDefaultLogOpts())
	require.NoError(t, err)
	p := &packetParserTCX{
		cfg: cfgPodLevelDisabled,
		l:   log.Logger().Named("test"),
	}
	err = p.Init()
	require.NoError(t, err)
}

func TestStartPodLevelDisabled(t *testing.T) {
	_, err := log.SetupZapLogger(log.GetDefaultLogOpts())
	require.NoError(t, err)
	p := &packetParserTCX{
		cfg: cfgPodLevelDisabled,
		l:   log.Logger().Named("test"),
	}
	ctx := context.Background()
	err = p.Start(ctx)
	require.NoError(t, err)
}

func TestCleanAll(t *testing.T) {
	_, err := log.SetupZapLogger(log.GetDefaultLogOpts())
	require.NoError(t, err)

	p := &packetParserTCX{
		cfg: cfgPodLevelEnabled,
		l:   log.Logger().Named("test"),
	}
	assert.NoError(t, p.cleanAll())

	p.tcxMap = &sync.Map{}
	assert.NoError(t, p.cleanAll())
}

func TestIsTCXSupported(t *testing.T) {
	// This test just verifies the function doesn't panic.
	// The actual result depends on kernel version.
	result := IsTCXSupported()
	assert.IsType(t, true, result)
}

func TestResolvePluginName(t *testing.T) {
	_, err := log.SetupZapLogger(log.GetDefaultLogOpts())
	require.NoError(t, err)

	p := New(cfgPodLevelEnabled)
	assert.Equal(t, "packetparsertcx", p.Name())

	p2 := New(cfgPodLevelDisabled)
	assert.Equal(t, "packetparsertcx", p2.Name())
}
