package kubernetes

import (
	"fmt"
	"os"

	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/kube"
)

type UninstallHelmChart struct {
	Namespace          string
	ReleaseName        string
	KubeConfigFilePath string
}

func (i *UninstallHelmChart) Run() error {
	settings := cli.New()
	settings.KubeConfig = i.KubeConfigFilePath
	actionConfig := new(action.Configuration)

	err := actionConfig.Init(settings.RESTClientGetter(), i.Namespace, os.Getenv("HELM_DRIVER"))
	if err != nil {
		return fmt.Errorf("failed to initialize helm action config: %w", err)
	}

	delclient := action.NewUninstall(actionConfig)
	delclient.WaitStrategy = kube.StatusWatcherStrategy
	delclient.Timeout = deleteTimeout
	_, err = delclient.Run(i.ReleaseName)
	if err != nil {
		return fmt.Errorf("failed to delete existing release %s: %w", i.ReleaseName, err)
	}

	return nil
}

func (i *UninstallHelmChart) Prevalidate() error {
	return nil
}

func (i *UninstallHelmChart) Stop() error {
	return nil
}
