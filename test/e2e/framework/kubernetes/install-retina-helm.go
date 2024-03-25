package kubernetes

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
)

const (
	createTimeout  = 240 * time.Second // windpws is slow
	deleteTimeout  = 60 * time.Second
	imageRegistry  = "acnpublic.azurecr.io"
	imageNamespace = "microsoft/retina"
)

var (
	errEmptyTag          = fmt.Errorf("tag is empty")
	errDirectoryNotFound = fmt.Errorf("directory not found")
)

type InstallHelmChart struct {
	Namespace          string
	ReleaseName        string
	KubeConfigFilePath string
	ChartPath          string
	TagEnv             string
}

func (i *InstallHelmChart) Run() error {
	settings := cli.New()
	settings.KubeConfig = i.KubeConfigFilePath
	actionConfig := new(action.Configuration)

	err := actionConfig.Init(settings.RESTClientGetter(), i.Namespace, os.Getenv("HELM_DRIVER"), log.Printf)
	if err != nil {
		return fmt.Errorf("failed to initialize helm action config: %w", err)
	}

	tag := os.Getenv(i.TagEnv)

	// load chart from the path
	chart, err := loader.Load(i.ChartPath)
	if err != nil {
		return fmt.Errorf("failed to load chart from path %s: %w", i.ChartPath, err)
	}

	chart.Values["imagePullSecrets"] = []map[string]interface{}{
		{
			"name": "acr-credentials",
		},
	}

	chart.Values["image"].(map[string]interface{})["tag"] = tag
	chart.Values["image"].(map[string]interface{})["pullPolicy"] = "Always"
	chart.Values["operator"].(map[string]interface{})["tag"] = tag

	// update helm chart to use images from acnrepo.azurecr.io when running e2e tests
	chart.Values["image"].(map[string]interface{})["repository"] = imageRegistry + "/" + imageNamespace + "/retina"
	chart.Values["image"].(map[string]interface{})["initRepository"] = imageRegistry + "/" + imageNamespace + "/retina-init"
	chart.Values["operator"].(map[string]interface{})["repository"] = imageRegistry + "/" + imageNamespace + "/retina-operator"

	getclient := action.NewGet(actionConfig)
	release, err := getclient.Run(i.ReleaseName)
	if err == nil && release != nil {
		log.Printf("found existing release by same name, removing before installing %s", release.Name)
		delclient := action.NewUninstall(actionConfig)
		delclient.Wait = true
		delclient.Timeout = deleteTimeout
		_, err = delclient.Run(i.ReleaseName)
		if err != nil {
			return fmt.Errorf("failed to delete existing release %s: %w", i.ReleaseName, err)
		}
	} else if err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("failed to get release %s: %w", i.ReleaseName, err)
	}

	client := action.NewInstall(actionConfig)
	client.Namespace = i.Namespace
	client.ReleaseName = i.ReleaseName
	client.Timeout = createTimeout
	client.Wait = true
	client.WaitForJobs = true

	// install the chart here
	rel, err := client.Run(chart, chart.Values)
	if err != nil {
		PrintPodLogs(i.KubeConfigFilePath, i.Namespace, "k8s-app=retina")
		return fmt.Errorf("failed to install chart: %w", err)
	}

	log.Printf("installed chart from path: %s in namespace: %s\n", rel.Name, rel.Namespace)
	// this will confirm the values set during installation
	log.Printf("chart values: %v\n", rel.Config)

	return nil
}

func (i *InstallHelmChart) Prevalidate() error {
	_, err := os.Stat(i.ChartPath)

	if os.IsNotExist(err) {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current working directory %s: %w", cwd, err)
		}
		log.Printf("the current working directory %s", cwd)
		return fmt.Errorf("directory not found at %s:  working directory: %s: %w", i.ChartPath, cwd, errDirectoryNotFound)
	}
	log.Printf("found chart at %s", i.ChartPath)

	if os.Getenv(i.TagEnv) == "" {
		return fmt.Errorf("tag is not set from env \"%s\": %w", i.TagEnv, errEmptyTag)
	}

	return nil
}

func (i *InstallHelmChart) Stop() error {
	return nil
}
