package conntrack

import (
	"context"
)

type Conntrack struct{}

// Not implemented for Windows
func New() *Conntrack {
	return &Conntrack{}
}

// Not implemented for Windows
func (c *Conntrack) Run(_ context.Context) error {
	return nil
}
