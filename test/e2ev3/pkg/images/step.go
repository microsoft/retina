package images

import (
	"context"

	"github.com/microsoft/retina/test/e2ev3/config"
)

// Step loads container images into the cluster.
type Step struct {
	Params *config.E2EParams
}

func (l *Step) String() string { return "load-images" }

func (l *Step) Do(ctx context.Context) error {
	p := l.Params
	return p.Loader.Load(ctx, RetinaImages(p.Cfg.Image.Registry, p.Cfg.Image.Namespace, p.Cfg.Image.Tag))
}
