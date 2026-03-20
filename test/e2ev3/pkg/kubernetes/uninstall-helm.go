package kubernetes

import (
	"context"
	"fmt"
	"log/slog"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
)

type UninstallHelmChart struct {
	Namespace          string
	ReleaseName        string
	KubeConfigFilePath string
	HelmDriver         string
}

func (i *UninstallHelmChart) String() string { return "uninstall-helm" }

func (i *UninstallHelmChart) Do(ctx context.Context) error {
	log := slog.With("step", i.String())
	settings := cli.New()
	settings.KubeConfig = i.KubeConfigFilePath
	actionConfig := new(action.Configuration)

	err := actionConfig.Init(settings.RESTClientGetter(), i.Namespace, i.HelmDriver, func(format string, v ...any) { log.Info(fmt.Sprintf(format, v...)) })
	if err != nil {
		return fmt.Errorf("failed to initialize helm action config: %w", err)
	}

	delclient := action.NewUninstall(actionConfig)
	delclient.Wait = true
	delclient.Timeout = deleteTimeout
	_, err = delclient.Run(i.ReleaseName)
	if err != nil {
		return fmt.Errorf("failed to delete existing release %s: %w", i.ReleaseName, err)
	}

	return nil
}
