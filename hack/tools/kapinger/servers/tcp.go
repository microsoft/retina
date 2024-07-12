package servers

import (
	"context"
	"fmt"
	"log"
	"net"
)

const (
	tcp = "tcp"
)

type KapingerTCPServer struct {
	port int
}

func NewKapingerTCPServer(port int) *KapingerTCPServer {
	return &KapingerTCPServer{
		port: port,
	}
}

func (k *KapingerTCPServer) Start(ctx context.Context) error {
	listener, err := net.ListenTCP(tcp, &net.TCPAddr{Port: k.port})
	if err != nil {
		fmt.Println(err)
		return nil
	}
	defer listener.Close()

	log.Printf("[TCP] Listening on %+v\n", listener.Addr().String())

	for {
		select {
		case <-ctx.Done():
			fmt.Println("Exiting TCP server")
			return nil
		default:
			connection, err := listener.Accept()
			if err != nil {
				fmt.Println(err)
				return err
			}
			handleConnection(connection)
		}
	}
}

func handleConnection(connection net.Conn) {
	addressString := fmt.Sprintf("%+v", connection.RemoteAddr())
	_, err := connection.Write(getResponse(addressString, tcp))
	if err != nil {
		fmt.Println(err)
	}

	err = connection.Close()
	if err != nil {
		fmt.Println(err)
	}
}
