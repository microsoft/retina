// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package controllermanager

import (
	"context"
	"fmt"

	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/controllers/cache"
	"github.com/microsoft/retina/pkg/enricher"

	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/managers/controllermanager/base"
	pm "github.com/microsoft/retina/pkg/managers/pluginmanager"
	sm "github.com/microsoft/retina/pkg/managers/servermanager"
	"github.com/microsoft/retina/pkg/pubsub"
	"github.com/microsoft/retina/pkg/telemetry"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
)

type StandardController struct {
	base.Controller
	conf   *kcfg.Config
	pubsub *pubsub.PubSub
}

func NewStandardControllerManager(conf *kcfg.Config, kubeclient kubernetes.Interface, tel telemetry.Telemetry) (*StandardController, error) {
	cmLogger := log.Logger().Named("standard-controller-manager")

	if conf.EnablePodLevel {
		// informer factory for pods/services
		factory := informers.NewSharedInformerFactory(kubeclient, base.ResyncTime)
		factory.WaitForCacheSync(wait.NeverStop)
	}

	pMgr, err := pm.NewPluginManager(conf, tel)
	if err != nil {
		return nil, fmt.Errorf("failed to create plugin manager: %w", err)
	}

	// create HTTP server for API server
	httpServer := sm.NewHTTPServer(
		conf.APIServer.Host,
		conf.APIServer.Port,
	)

	return &StandardController{
		Controller: base.Controller{
			L:             cmLogger,
			HTTPServer:    httpServer,
			PluginManager: pMgr,
			Tel:           tel,
		},
		conf: conf,
	}, nil
}

func (m *StandardController) Init(ctx context.Context) error {
	m.L.Info("Initializing standard controller manager ...")

	if err := m.HTTPServer.Init(); err != nil {
		return fmt.Errorf("failed to initialize HTTP server: %w", err)
	}

	if m.conf.EnablePodLevel {
		// create pubsub instance
		m.pubsub = pubsub.New()

		// create cache instance
		m.Cache = cache.New(m.pubsub)

		// create enricher instance
		m.Enricher = enricher.NewStandard(ctx, m.Cache)
	}

	return nil
}
