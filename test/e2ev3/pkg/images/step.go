package images

import (
	"context"

	"github.com/microsoft/retina/test/e2ev3/config"
)

// Step loads container images into the cluster.
type Step struct {
	Cfg *config.E2EConfig
}

func (l *Step) String() string { return "load-images" }

func (l *Step) Do(ctx context.Context) error {
	p := l.Cfg
	loader := NewLoader(*config.Provider, p.Azure.ClusterName)
	return loader.Load(ctx, RetinaImages(p.Image.Registry, p.Image.Namespace, p.Image.Tag))
}
