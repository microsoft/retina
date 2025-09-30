// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package controllermanager

import (
	"testing"

	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/telemetry"

	"github.com/stretchr/testify/require"
)

const (
	testStandaloneCfgFile = "../../config/testwith/config-standalone.yaml"
)

func TestNewStandaloneControllerManager(t *testing.T) {
	c, err := kcfg.GetStandaloneConfig(testStandaloneCfgFile)
	require.NoError(t, err, "Expected no error, instead got %+v", err)
	require.NotNil(t, c)

	if _, err = log.SetupZapLogger(log.GetDefaultLogOpts()); err != nil {
		t.Errorf("Error setting up logger: %s", err)
	}

	cm, err := NewStandaloneControllerManager(c, telemetry.NewNoopTelemetry())
	require.Error(t, err, "Expected error of not recognising windows plugins in linux, instead got no error")
	require.Nil(t, cm)
}
