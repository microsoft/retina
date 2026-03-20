// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package load

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
)

// Kind loads images directly onto Kind cluster nodes using `kind load docker-image`.
type Kind struct {
	ClusterName string
}

func (k *Kind) Load(ctx context.Context, images []string) error {
	for _, image := range images {
		log.Printf("loading image %s onto kind cluster %q", image, k.ClusterName)
		args := []string{"load", "docker-image", "--name", k.ClusterName, image}
		cmd := exec.CommandContext(ctx, "kind", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("kind load docker-image %s: %w", image, err)
		}
	}
	return nil
}

func (k *Kind) PullPolicy() string {
	return "IfNotPresent"
}

func (k *Kind) PullSecrets() []map[string]interface{} {
	return nil
}
