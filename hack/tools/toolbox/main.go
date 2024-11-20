package main

import (
	"context"

	"github.com/microsoft/retina/hack/tools/kapinger/config"
	"github.com/microsoft/retina/hack/tools/toolbox/kapinger/servers"
)

func main() {
	cfg := config.LoadConfigFromEnv()

	ctx := context.Background()
	servers.StartAll(ctx, cfg)
}
