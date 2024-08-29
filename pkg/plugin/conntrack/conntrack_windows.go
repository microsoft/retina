package conntrack

import (
	"context"
<<<<<<< HEAD
=======

	"github.com/microsoft/retina/pkg/config"
>>>>>>> main
)

type Conntrack struct{}

// Not implemented for Windows
<<<<<<< HEAD
func New() *Conntrack {
=======
func New(_ *config.Config) *Conntrack {
>>>>>>> main
	return &Conntrack{}
}

// Not implemented for Windows
func (c *Conntrack) Run(_ context.Context) error {
	return nil
}
