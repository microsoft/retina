// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package controllermanager

import (
	"fmt"

	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/managers/controllermanager/base"
	pm "github.com/microsoft/retina/pkg/managers/pluginmanager"
	sm "github.com/microsoft/retina/pkg/managers/servermanager"
	"github.com/microsoft/retina/pkg/telemetry"
)

type StandaloneController struct {
	base.Controller
	conf *kcfg.StandaloneConfig
}

func NewStandaloneControllerManager(conf *kcfg.StandaloneConfig, tel telemetry.Telemetry) (*StandaloneController, error) {
	cmLogger := log.Logger().Named("standalone-controller-manager")

	pMgr, err := pm.NewPluginManager(kcfg.StandaloneConfigAdapter(conf), tel)
	if err != nil {
		return nil, fmt.Errorf("failed to create plugin manager: %w", err)
	}

	// create HTTP server for API server
	httpServer := sm.NewHTTPServer(
		conf.APIServer.Host,
		conf.APIServer.Port,
	)

	return &StandaloneController{
		Controller: base.Controller{
			L:             cmLogger,
			HTTPServer:    httpServer,
			PluginManager: pMgr,
			Tel:           tel,
		},
		conf: conf,
	}, nil
}

func (m *StandaloneController) Init() error {
	m.L.Info("Initializing standalone controller manager")

	if err := m.HTTPServer.Init(); err != nil {
		return fmt.Errorf("failed to initialize HTTP server: %w", err)
	}

	return nil
}
