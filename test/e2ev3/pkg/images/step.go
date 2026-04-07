package images

import (
	"context"

	"github.com/microsoft/retina/test/e2ev3/config"
	"github.com/microsoft/retina/test/e2ev3/pkg/utils"
)

// Step loads container images into the cluster.
type Step struct {
	Cfg *config.E2EConfig
}

func (l *Step) String() string { return "load-images" }

func (l *Step) Do(ctx context.Context) error {
	ctx, log := utils.StepLogger(ctx, l)
	p := l.Cfg
	imgs := RetinaImages(p.Image.Registry, p.Image.Namespace, p.Image.Tag)
	log.Info("loading images into cluster", "count", len(imgs), "cluster", p.Cluster.ClusterName())
	return p.Cluster.LoadImages(ctx, imgs)
}

// RetinaImages returns the standard Retina image references for the given coordinates.
func RetinaImages(registry, namespace, tag string) []string {
	base := registry + "/" + namespace
	return []string{
		base + "/retina-agent:" + tag,
		base + "/retina-init:" + tag,
		base + "/retina-operator:" + tag,
	}
}
