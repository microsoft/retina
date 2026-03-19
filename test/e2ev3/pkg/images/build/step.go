//go:build e2e

package build

import (
	"context"

	"github.com/microsoft/retina/test/e2ev3/config"
)

// Step builds container images from source.
type Step struct {
	Params *config.E2EParams
}

func (b *Step) String() string { return "build-images" }

func (b *Step) Do(ctx context.Context) error {
	p := b.Params
	return (&Images{
		RootDir:   p.Paths.RootDir,
		Registry:  p.Cfg.Image.Registry,
		Namespace: p.Cfg.Image.Namespace,
		Tag:       p.Cfg.Image.Tag,
		Push:      *config.Provider != "kind",
	}).Do(ctx)
}
