// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package kind

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"

	"github.com/microsoft/retina/test/e2ev3/pkg/utils"
	"k8s.io/client-go/rest"
)

// Cluster is a ClusterProvider for Kind (Kubernetes in Docker) clusters.
// Images are loaded directly onto cluster nodes via `kind load docker-image`.
type Cluster struct {
	Name       string
	KubeCfgPath string
	RC          *rest.Config
}

func (k *Cluster) ClusterName() string            { return k.Name }
func (k *Cluster) KubeConfigPath() string          { return k.KubeCfgPath }
func (k *Cluster) RestConfig() *rest.Config        { return k.RC }

func (k *Cluster) LoadImages(ctx context.Context, images []string) error {
	prefix := utils.Prefix(ctx)
	for _, image := range images {
		slog.With("prefix", prefix).Info("loading image onto kind cluster", "image", image, "cluster", k.Name)
		args := []string{"load", "docker-image", "--name", k.Name, image}
		cmd := exec.CommandContext(ctx, "kind", args...)
		cmdOut := &utils.SlogWriter{Level: slog.LevelInfo, Source: prefix}
		cmd.Stdout = cmdOut
		cmd.Stderr = cmdOut
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("kind load docker-image %s: %w", image, err)
		}
		cmdOut.Flush()
	}
	return nil
}

func (k *Cluster) ImagePullPolicy() string                    { return "IfNotPresent" }
func (k *Cluster) ImagePullSecrets() []map[string]interface{} { return nil }
