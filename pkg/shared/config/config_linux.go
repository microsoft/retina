package config

import (
	"fmt"

	"github.com/cilium/hive/cell"
	"k8s.io/client-go/rest"
	kcfg "sigs.k8s.io/controller-runtime/pkg/client/config"
)

var Cell = cell.Module(
	"shared-config",
	"Shared Config",
	cell.Provide(GetK8sConfig),
)

func GetK8sConfig() (*rest.Config, error) {
	k8sCfg, err := kcfg.GetConfig()
	if err != nil {
		return &rest.Config{}, fmt.Errorf("failed to get k8s config: %w", err)
	}
	return k8sCfg, nil
}
