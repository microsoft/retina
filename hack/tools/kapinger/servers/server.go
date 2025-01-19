package servers

import (
	"context"
	"log"

	"github.com/microsoft/retina/hack/tools/kapinger/config"
)

type Server interface {
	Start(ctx context.Context) error
}

type Kapinger struct {
	servers []Server
}

func (k *Kapinger) Start(ctx context.Context) {
	for i := range k.servers {
		go func(i int) {
			err := k.servers[i].Start(ctx)
			if err != nil {
				log.Printf("Error starting server: %s\n", err)
			}
		}(i)
	}
	<-ctx.Done()
}

func StartAll(ctx context.Context, config *config.KapingerConfig) {
	k := &Kapinger{
		servers: []Server{
			NewKapingerTCPServer(config.TCPPort),
			NewKapingerUDPServer(config.UDPPort),
			NewKapingerHTTPServer(config.HTTPPort),
		},
	}

	// cancel
	k.Start(ctx)
}
