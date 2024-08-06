package client

import (
	"fmt"
	"time"

	observerv1 "github.com/cilium/cilium/api/v1/observer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	observerv1.ObserverClient
}

func New() (*Client, error) {
	// TODO rethink the dial opts
	// starting with opts seen at https://github.com/cilium/hubble-ui/blob/a06e19ba65299c63a58034a360aeedde9266ec01/backend/internal/relay_client/connection_props.go#L34-L70
	connectParams := grpc.ConnectParams{
		Backoff: backoff.Config{
			BaseDelay:  1.0 * time.Second,
			Multiplier: 1.6,
			Jitter:     0.2,
			MaxDelay:   7 * time.Second,
		},
		MinConnectTimeout: 5 * time.Second,
	}
	connectDialOption := grpc.WithConnectParams(connectParams)

	tlsDialOption := grpc.WithTransportCredentials(insecure.NewCredentials())

	// FIXME make address part of a config
	addr := ":5557"
	connection, err := grpc.NewClient(addr, tlsDialOption, connectDialOption)
	if err != nil {
		return nil, fmt.Errorf("failed to dial %s: %w", addr, err)
	}

	client := &Client{
		ObserverClient: observerv1.NewObserverClient(connection),
	}
	return client, nil
}
