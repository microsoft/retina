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
	createTimeout = 20 * time.Minute // windows is slow
	deleteTimeout = 5 * time.Minute
)

var (
	errEmpty                 = errors.New("is empty")
	errDirectoryNotFound     = errors.New("directory not found")
	errUnexpectedReleaseType = errors.New("unexpected release type")
)

type InstallHelmChart struct {
	Namespace          string
	ReleaseName        string
	KubeConfigFilePath string
	ChartPath          string
	TagEnv             string
	EnableHeartbeat    bool
}

func (i *InstallHelmChart) Run() error {
	ctx, cancel := context.WithTimeout(context.Background(), createTimeout)
	defer cancel()
	settings := cli.New()
	settings.KubeConfig = i.KubeConfigFilePath
	actionConfig := new(action.Configuration)

	err := actionConfig.Init(settings.RESTClientGetter(), i.Namespace, os.Getenv("HELM_DRIVER"))
	if err != nil {
		return fmt.Errorf("failed to initialize helm action config: %w", err)
	}

	// Creating extra namespace to deploy test pods
	err = CreateNamespaceFn(i.KubeConfigFilePath, common.TestPodNamespace)
	if err != nil {
		return fmt.Errorf("failed to create namespace %s: %w", i.Namespace, err)
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

	//Download necessary CRD's
	err = downloadExternalCRDs(i.ChartPath)
	if err != nil {
		return fmt.Errorf("failed to load external crd's: %w", err)
	}

	// load chart from the path
	chart, err := loader.Load(i.ChartPath)
	if err != nil {
		return fmt.Errorf("failed to load chart from path %s: %w", i.ChartPath, err)
	}

	// value overrides; helm merges these onto the chart defaults
	overrides := map[string]any{
		"imagePullSecrets": []map[string]any{
			{"name": "acr-credentials"},
		},
		"image": map[string]any{
			"repository":     imageRegistry + "/" + imageNamespace + "/retina-agent",
			"initRepository": imageRegistry + "/" + imageNamespace + "/retina-init",
			"tag":            tag,
			"pullPolicy":     "Always",
		},
		"operator": map[string]any{
			"enabled":    true,
			"repository": imageRegistry + "/" + imageNamespace + "/retina-operator",
			"tag":        tag,
		},
	}

	if i.EnableHeartbeat {
		overrides["enableTelemetry"] = i.EnableHeartbeat
		overrides["logLevel"] = "error"
	}

	getclient := action.NewGet(actionConfig)
	_, err = getclient.Run(i.ReleaseName)
	switch {
	case err == nil:
		log.Printf("found existing release by same name, removing before installing %s", i.ReleaseName)
		delclient := action.NewUninstall(actionConfig)
		delclient.WaitStrategy = kube.StatusWatcherStrategy
		delclient.Timeout = deleteTimeout
		_, err = delclient.Run(i.ReleaseName)
		if err != nil {
			return fmt.Errorf("failed to delete existing release %s: %w", i.ReleaseName, err)
		}
	case !errors.Is(err, driver.ErrReleaseNotFound):
		return fmt.Errorf("failed to get release %s: %w", i.ReleaseName, err)
	}

	client := action.NewInstall(actionConfig)
	client.Namespace = i.Namespace
	client.ReleaseName = i.ReleaseName
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
	config, err := clientcmd.BuildConfigFromFlags("", i.KubeConfigFilePath)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("error creating Kubernetes client: %w", err)
	}

	labelSelector := "k8s-app=retina"
	err = WaitForPodReady(ctx, clientset, "kube-system", labelSelector)
	if err != nil {
		return fmt.Errorf("error waiting for retina pods to be ready: %w", err)
	}

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
		return fmt.Errorf("tag is not set from env \"%s\": %w", i.TagEnv, errEmpty)
	}

	return nil
}

func (i *InstallHelmChart) Stop() error {
	return nil
}
