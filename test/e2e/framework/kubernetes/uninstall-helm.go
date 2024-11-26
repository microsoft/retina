package kubernetes

import (
	"fmt"
	"log"
	"os"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
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

	err := actionConfig.Init(settings.RESTClientGetter(), i.Namespace, os.Getenv("HELM_DRIVER"), log.Printf)
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

func (i *UninstallHelmChart) Prevalidate() error {
	return nil
}

func (i *UninstallHelmChart) Stop() error {
	return nil
}
