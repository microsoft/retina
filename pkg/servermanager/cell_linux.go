package servermanager

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/cilium/hive/cell"
	"github.com/microsoft/retina/pkg/config"
	sm "github.com/microsoft/retina/pkg/managers/servermanager"
)

var Cell = cell.Module(
	"servermanager",
	"Manages Retina basic metrics server",
	cell.Provide(newServerManager),
)

type serverParams struct {
	cell.In

	Log       *slog.Logger
	Lifecycle cell.Lifecycle
	Config    config.Config
}

func newServerManager(params serverParams) (*sm.HTTPServer, error) {
	logger := params.Log.With("module", "servermanager")

	serverCtx, cancelCtx := context.WithCancel(context.Background())
	serverManager := sm.NewHTTPServer(params.Config.APIServer.Host, params.Config.APIServer.Port)
	if err := serverManager.Init(); err != nil {
		logger.Error("Unable to initialize Http server", "error", err)
		cancelCtx()
		return nil, fmt.Errorf("unable to initialize Http server: %w", err)
	}

	wg := sync.WaitGroup{}
	params.Lifecycle.Append(cell.Hook{
		OnStart: func(cell.HookContext) error {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := serverManager.Start(serverCtx); err != nil {
					logger.Error("Unable to start server", "error", err)
				}
			}()

			return nil
		},
		OnStop: func(cell.HookContext) error {
			cancelCtx()
			wg.Wait()
			return nil
		},
	})

	return serverManager, nil
}
