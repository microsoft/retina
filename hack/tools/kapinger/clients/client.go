package clients

import (
	"context"
)

type Client interface {
	MakeRequests(ctx context.Context) error
}
