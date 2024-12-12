package kubernetes

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/microsoft/retina/test/e2e/common"
	generic "github.com/microsoft/retina/test/e2e/framework/generic"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	HubbleNamespace = "kube-system"
	HubbleUIApp     = "hubble-ui"
	HubbleRelayApp  = "hubble-relay"
)

type InstallHubbleHelmChart struct {
	Namespace          string
	ReleaseName        string
	KubeConfigFilePath string
	ChartPath          string
	TagEnv             string
}

func (v *InstallHubbleHelmChart) Run() error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeoutSeconds*time.Second)
	defer cancel()

	settings := cli.New()
	settings.KubeConfig = v.KubeConfigFilePath
	actionConfig := new(action.Configuration)

	err := actionConfig.Init(settings.RESTClientGetter(), v.Namespace, os.Getenv("HELM_DRIVER"), log.Printf)
	if err != nil {
		return fmt.Errorf("failed to initialize helm action config: %w", err)
	}

	// Creating extra namespace to deploy test pods
	err = CreateNamespaceFn(v.KubeConfigFilePath, common.TestPodNamespace)
	if err != nil {
		return fmt.Errorf("failed to create namespace %s: %w", v.Namespace, err)
	}

	tag := os.Getenv(generic.DefaultTagEnv)
	if tag == "" {
		return fmt.Errorf("tag is not set: %w", errEmpty)
	}
	imageRegistry := os.Getenv(generic.DefaultImageRegistry)
	if imageRegistry == "" {
		return fmt.Errorf("image registry is not set: %w", errEmpty)
	}

	imageNamespace := os.Getenv(generic.DefaultImageNamespace)
	if imageNamespace == "" {
		return fmt.Errorf("image namespace is not set: %w", errEmpty)
	}

	// load chart from the path
	chart, err := loader.Load(v.ChartPath)
	if err != nil {
		return fmt.Errorf("failed to load chart from path %s: %w", v.ChartPath, err)
	}

	chart.Values["imagePullSecrets"] = []map[string]interface{}{
		{
			"name": "acr-credentials",
		},
	}
	chart.Values["operator"].(map[string]interface{})["enabled"] = true
	chart.Values["operator"].(map[string]interface{})["repository"] = imageRegistry + "/" + imageNamespace + "/retina-operator"
	chart.Values["operator"].(map[string]interface{})["tag"] = tag
	chart.Values["agent"].(map[string]interface{})["enabled"] = true
	chart.Values["agent"].(map[string]interface{})["repository"] = imageRegistry + "/" + imageNamespace + "/retina-agent"
	chart.Values["agent"].(map[string]interface{})["tag"] = tag
	chart.Values["agent"].(map[string]interface{})["init"].(map[string]interface{})["enabled"] = true
	chart.Values["agent"].(map[string]interface{})["init"].(map[string]interface{})["repository"] = imageRegistry + "/" + imageNamespace + "/retina-init"
	chart.Values["agent"].(map[string]interface{})["init"].(map[string]interface{})["tag"] = tag
	chart.Values["hubble"].(map[string]interface{})["tls"].(map[string]interface{})["enabled"] = false
	chart.Values["hubble"].(map[string]interface{})["relay"].(map[string]interface{})["tls"].(map[string]interface{})["server"].(map[string]interface{})["enabled"] = false
	chart.Values["hubble"].(map[string]interface{})["tls"].(map[string]interface{})["auto"].(map[string]interface{})["enabled"] = false

	getclient := action.NewGet(actionConfig)
	release, err := getclient.Run(v.ReleaseName)
	if err == nil && release != nil {
		log.Printf("found existing release by same name, removing before installing %s", release.Name)
		delclient := action.NewUninstall(actionConfig)
		delclient.Wait = true
		delclient.Timeout = deleteTimeout
		_, err = delclient.Run(v.ReleaseName)
		if err != nil {
			return fmt.Errorf("failed to delete existing release %s: %w", v.ReleaseName, err)
		}
	} else if err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("failed to get release %s: %w", v.ReleaseName, err)
	}

	client := action.NewInstall(actionConfig)
	client.Namespace = v.Namespace
	client.ReleaseName = v.ReleaseName
	client.Timeout = createTimeout
	client.Wait = true
	client.WaitForJobs = true

	// install the chart here
	rel, err := client.RunWithContext(ctx, chart, chart.Values)
	if err != nil {
		return fmt.Errorf("failed to install chart: %w", err)
	}

	log.Printf("installed chart from path: %s in namespace: %s\n", rel.Name, rel.Namespace)
	// this will confirm the values set during installation
	log.Printf("chart values: %v\n", rel.Config)

	// ensure all pods are running, since helm doesn't care about windows
	config, err := clientcmd.BuildConfigFromFlags("", v.KubeConfigFilePath)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("error creating Kubernetes client: %w", err)
	}

	// Validate Hubble Relay Pod
	if err := WaitForPodReady(ctx, clientset, HubbleNamespace, "k8s-app="+HubbleRelayApp); err != nil {
		return fmt.Errorf("error waiting for Hubble Relay pods to be ready: %w", err)
	}
	log.Printf("Hubble Relay Pod is ready")

	// Validate Hubble UI Pod
	if err := WaitForPodReady(ctx, clientset, HubbleNamespace, "k8s-app="+HubbleUIApp); err != nil {
		return fmt.Errorf("error waiting for Hubble UI pods to be ready: %w", err)
	}
	log.Printf("Hubble UI Pod is ready")

	return nil
}

func (v *InstallHubbleHelmChart) Prevalidate() error {
	return nil
}

func (v *InstallHubbleHelmChart) Stop() error {
	return nil
}
