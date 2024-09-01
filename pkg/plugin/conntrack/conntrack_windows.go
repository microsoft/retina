package conntrack

import (
	"context"

	"github.com/microsoft/retina/pkg/config"
)

type Conntrack struct{}

// Not implemented for Windows
func New(_ *config.Config) *Conntrack {
	return &Conntrack{}
}

// Not implemented for Windows
func (c *Conntrack) Run(_ context.Context) error {
	return nil
}
