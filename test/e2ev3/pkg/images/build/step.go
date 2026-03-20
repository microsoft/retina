//go:build e2e

package build

import (
	"context"

	"github.com/microsoft/retina/test/e2ev3/config"
)

// Step builds container images from source.
type Step struct {
	Cfg *config.E2EConfig
}

func (b *Step) String() string { return "build-images" }

func (b *Step) Do(ctx context.Context) error {
	p := b.Cfg
	return (&Images{
		RootDir:   p.Paths.RootDir,
		Registry:  p.Image.Registry,
		Namespace: p.Image.Namespace,
		Tag:       p.Image.Tag,
		Push:      *config.Provider != "kind",
	}).Do(ctx)
}
