package servers

import (
	"context"
	"log"
	"os"
	"strconv"
)

const (
	HTTPPort = 8080
	TCPPort  = 8085
	UDPPort  = 8086

	EnvHTTPPort = "HTTP_PORT"
	EnvTCPPort  = "TCP_PORT"
	EnvUDPPort  = "UDP_PORT"
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

func StartAll() {
	tcpPort, err := strconv.Atoi(os.Getenv(EnvTCPPort))
	if err != nil {
		tcpPort = TCPPort
		log.Printf("TCP_PORT not set, defaulting to port %d\n", TCPPort)
	}

	udpPort, err := strconv.Atoi(os.Getenv(EnvUDPPort))
	if err != nil {
		udpPort = UDPPort
		log.Printf("UDP_PORT not set, defaulting to port %d\n", UDPPort)
	}

	httpPort, err := strconv.Atoi(os.Getenv(EnvHTTPPort))
	if err != nil {
		httpPort = HTTPPort
		log.Printf("HTTP_PORT not set, defaulting to port %d\n", HTTPPort)
	}

	k := &Kapinger{
		servers: []Server{
			NewKapingerTCPServer(tcpPort),
			NewKapingerUDPServer(udpPort),
			NewKapingerHTTPServer(httpPort),
		},
	}

	// cancel
	ctx := context.Background()
	k.Start(ctx)
}
