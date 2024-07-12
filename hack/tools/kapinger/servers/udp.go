package servers

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"
)

const (
	udp = "udp"
)

type KapingerUDPServer struct {
	buffersize int
	port       int
}

func NewKapingerUDPServer(port int) *KapingerUDPServer {
	return &KapingerUDPServer{
		buffersize: 1024,
		port:       port,
	}
}

func (k *KapingerUDPServer) Start(ctx context.Context) error {
	connection, err := net.ListenUDP(udp, &net.UDPAddr{Port: k.port})
	if err != nil {
		fmt.Println(err)
		return err
	}
	log.Printf("[UDP] Listening on %+v\n", connection.LocalAddr().String())

	defer connection.Close()
	buffer := make([]byte, k.buffersize)

	for {
		select {
		case <-ctx.Done():
			fmt.Println("Exiting UDP server")
			return nil
		default:
			n, addr, err := connection.ReadFromUDP(buffer)
			if err != nil {
				fmt.Println(err)
			}
			payload := strings.TrimSpace(string(buffer[0 : n-1]))

			if payload == "STOP" {
				fmt.Println("Exiting UDP server")
				return nil
			}

			addressString := fmt.Sprintf("%+v", addr)
			_, err = connection.WriteToUDP(getResponse(addressString, udp), addr)
			if err != nil {
				return fmt.Errorf("error writing to UDP connection: %w", err)
			}
		}
	}
}
