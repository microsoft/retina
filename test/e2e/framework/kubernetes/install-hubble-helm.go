package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/microsoft/retina/test/e2e/common"
	generic "github.com/microsoft/retina/test/e2e/framework/generic"
	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/chart/loader"
	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/kube"
	releasev1 "helm.sh/helm/v4/pkg/release/v1"
	"helm.sh/helm/v4/pkg/storage/driver"
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

	err := actionConfig.Init(settings.RESTClientGetter(), v.Namespace, os.Getenv("HELM_DRIVER"))
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

	// value overrides; helm merges these onto the chart defaults
	overrides := map[string]any{
		"imagePullSecrets": []map[string]any{
			{"name": "acr-credentials"},
		},
		"operator": map[string]any{
			"enabled":    true,
			"repository": imageRegistry + "/" + imageNamespace + "/retina-operator",
			"tag":        tag,
		},
		"agent": map[string]any{
			"enabled":    true,
			"repository": imageRegistry + "/" + imageNamespace + "/retina-agent",
			"tag":        tag,
			"init": map[string]any{
				"enabled":    true,
				"repository": imageRegistry + "/" + imageNamespace + "/retina-init",
				"tag":        tag,
			},
		},
		"hubble": map[string]any{
			"tls": map[string]any{
				"enabled": false,
				"auto": map[string]any{
					"enabled": false,
				},
			},
			"relay": map[string]any{
				"tls": map[string]any{
					"server": map[string]any{
						"enabled": false,
					},
				},
			},
		},
	}

	getclient := action.NewGet(actionConfig)
	_, err = getclient.Run(v.ReleaseName)
	switch {
	case err == nil:
		log.Printf("found existing release by same name, removing before installing %s", v.ReleaseName)
		delclient := action.NewUninstall(actionConfig)
		delclient.WaitStrategy = kube.StatusWatcherStrategy
		delclient.Timeout = deleteTimeout
		_, err = delclient.Run(v.ReleaseName)
		if err != nil {
			return fmt.Errorf("failed to delete existing release %s: %w", v.ReleaseName, err)
		}
	case !errors.Is(err, driver.ErrReleaseNotFound):
		return fmt.Errorf("failed to get release %s: %w", v.ReleaseName, err)
	}

	client := action.NewInstall(actionConfig)
	client.Namespace = v.Namespace
	client.ReleaseName = v.ReleaseName
	client.Timeout = createTimeout
	client.WaitStrategy = kube.StatusWatcherStrategy
	client.WaitForJobs = true

	// install the chart here
	rel, err := client.RunWithContext(ctx, chart, overrides)
	if err != nil {
		return fmt.Errorf("failed to install chart: %w", err)
	}
	release, ok := rel.(*releasev1.Release)
	if !ok {
		return fmt.Errorf("%w: %T", errUnexpectedReleaseType, rel)
	}

	log.Printf("installed chart from path: %s in namespace: %s\n", release.Name, release.Namespace)
	// this will confirm the values set during installation
	log.Printf("chart values: %v\n", release.Config)

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
