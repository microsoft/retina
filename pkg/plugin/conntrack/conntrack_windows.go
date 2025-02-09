package conntrack

import (
	"context"

	"github.com/microsoft/retina/pkg/config"
)

type Conntrack struct{}

// Not implemented for Windows
func New() (*Conntrack, error) {
	return &Conntrack{}, nil
}

// Not implemented for Windows
func (c *Conntrack) Run(_ context.Context) error {
	return nil
}

// SetConfig sets the config after initialization
func (c *Conntrack) SetConfig(_ *config.Config) {
	// No-op for Windows
}
