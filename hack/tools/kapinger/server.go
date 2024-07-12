package main

import (
	"context"
	"log"
	"os"
	"strconv"

	"github.com/microsoft/retina/hack/tools/kapinger/servers"
)

const (
	httpport = 8080
	tcpport  = 8085
	udpport  = 8086

	envHTTPPort = "HTTP_PORT"
	envTCPPort  = "TCP_PORT"
	envUDPPort  = "UDP_PORT"
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

func StartServers() {
	tcpPort, err := strconv.Atoi(os.Getenv(envTCPPort))
	if err != nil {
		tcpPort = tcpport
		log.Printf("TCP_PORT not set, defaulting to port %d\n", tcpport)
	}

	udpPort, err := strconv.Atoi(os.Getenv(envUDPPort))
	if err != nil {
		udpPort = udpport
		log.Printf("UDP_PORT not set, defaulting to port %d\n", udpport)
	}

	httpPort, err := strconv.Atoi(os.Getenv(envHTTPPort))
	if err != nil {
		httpPort = httpport
		log.Printf("HTTP_PORT not set, defaulting to port %d\n", httpport)
	}

	k := &Kapinger{
		servers: []Server{
			servers.NewKapingerTCPServer(tcpPort),
			servers.NewKapingerUDPServer(udpPort),
			servers.NewKapingerHTTPServer(httpPort),
		},
	}

	// cancel
	ctx := context.Background()
	k.Start(ctx)
}
