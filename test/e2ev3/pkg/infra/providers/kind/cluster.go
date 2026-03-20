// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package kind

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"

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
	for _, image := range images {
		log.Printf("loading image %s onto kind cluster %q", image, k.Name)
		args := []string{"load", "docker-image", "--name", k.Name, image}
		cmd := exec.CommandContext(ctx, "kind", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("kind load docker-image %s: %w", image, err)
		}
	}
	return nil
}

func (k *Cluster) ImagePullPolicy() string                    { return "IfNotPresent" }
func (k *Cluster) ImagePullSecrets() []map[string]interface{} { return nil }
