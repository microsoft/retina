package kubernetes

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	e2ecfg "github.com/microsoft/retina/test/e2ev3/config"
	"github.com/microsoft/retina/test/e2ev3/pkg/utils"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	helmValues "helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/getter"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	createTimeout = 20 * time.Minute // windows is slow
	deleteTimeout = 5 * time.Minute
)

var (
	errEmpty             = fmt.Errorf("is empty")
	errDirectoryNotFound = fmt.Errorf("directory not found")
)

type InstallHelmChart struct {
	Namespace          string
	ReleaseName        string
	KubeConfigFilePath string
	ChartPath          string
	ValuesFile         string
	ImageTag           string
	ImageRegistry      string
	ImageNamespace     string
	HelmDriver         string
	ImageLoader        e2ecfg.ClusterProvider
	EnableHeartbeat    bool
	TestPodNamespace   string
}

func (i *InstallHelmChart) Do(ctx context.Context) error {
	ctx, log := utils.StepLogger(ctx, i)
	// Prevalidation: check chart path and tag env
	_, err := os.Stat(i.ChartPath)
	if os.IsNotExist(err) {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current working directory %s: %w", cwd, err)
		}
		log.Info("current working directory", "cwd", cwd)
		return fmt.Errorf("directory not found at %s:  working directory: %s: %w", i.ChartPath, cwd, errDirectoryNotFound)
	}
	log.Info("found chart", "path", i.ChartPath)

	if i.ImageTag == "" {
		return fmt.Errorf("image tag is not set: %w", errEmpty)
	}
	if i.ImageRegistry == "" {
		return fmt.Errorf("image registry is not set: %w", errEmpty)
	}
	if i.ImageNamespace == "" {
		return fmt.Errorf("image namespace is not set: %w", errEmpty)
	}

	tag := i.ImageTag
	imageRegistry := i.ImageRegistry
	imageNamespace := i.ImageNamespace

	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()
	settings := cli.New()
	settings.KubeConfig = i.KubeConfigFilePath
	actionConfig := new(action.Configuration)

	err = actionConfig.Init(settings.RESTClientGetter(), i.Namespace, i.HelmDriver, func(format string, v ...any) { log.Info(fmt.Sprintf(format, v...)) })
	if err != nil {
		return fmt.Errorf("failed to initialize helm action config: %w", err)
	}

	// Creating extra namespace to deploy test pods
	rc, err := clientcmd.BuildConfigFromFlags("", i.KubeConfigFilePath)
	if err != nil {
		return fmt.Errorf("failed to build rest config: %w", err)
	}
	testNS := i.TestPodNamespace
	if testNS == "" {
		testNS = e2ecfg.TestPodNamespace
	}
	err = CreateNamespaceFn(ctx, rc, testNS)
	if err != nil {
		return fmt.Errorf("failed to create namespace %s: %w", i.Namespace, err)
	}

	//Download necessary CRD's
	err = downloadExternalCRDs(ctx, i.ChartPath)
	if err != nil {
		return fmt.Errorf("failed to load external crd's: %w", err)
	}

	// load chart from the path
	chart, err := loader.Load(i.ChartPath)
	if err != nil {
		return fmt.Errorf("failed to load chart from path %s: %w", i.ChartPath, err)
	}

	// merge values from an optional profile file
	if i.ValuesFile != "" {
		options := helmValues.Options{
			ValueFiles: []string{i.ValuesFile},
		}
		provider := getter.All(settings)
		overrides, mergeErr := options.MergeValues(provider)
		if mergeErr != nil {
			return fmt.Errorf("failed to merge values from %s: %w", i.ValuesFile, mergeErr)
		}
		for k, v := range overrides {
			chart.Values[k] = v
		}
		log.Info("applied values file", "file", i.ValuesFile, "overrides", overrides)
	}

	if secrets := i.ImageLoader.ImagePullSecrets(); len(secrets) > 0 {
		chart.Values["imagePullSecrets"] = secrets
	}

	if i.EnableHeartbeat {
		chart.Values["enableTelemetry"] = i.EnableHeartbeat
		chart.Values["logLevel"] = "error"
	}

	chart.Values["image"].(map[string]interface{})["tag"] = tag
	chart.Values["image"].(map[string]interface{})["pullPolicy"] = i.ImageLoader.ImagePullPolicy()
	chart.Values["operator"].(map[string]interface{})["tag"] = tag
	chart.Values["image"].(map[string]interface{})["repository"] = imageRegistry + "/" + imageNamespace + "/retina-agent"
	chart.Values["image"].(map[string]interface{})["initRepository"] = imageRegistry + "/" + imageNamespace + "/retina-init"
	chart.Values["operator"].(map[string]interface{})["repository"] = imageRegistry + "/" + imageNamespace + "/retina-operator"
	chart.Values["operator"].(map[string]interface{})["enabled"] = true

	getclient := action.NewGet(actionConfig)
	release, err := getclient.Run(i.ReleaseName)
	if err == nil && release != nil {
		log.Info("found existing release, removing before installing", "release", release.Name)
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
	rel, err := client.RunWithContext(ctx, chart, chart.Values)
	if err != nil {
		return fmt.Errorf("failed to install chart: %w", err)
	}

	log.Info("installed chart", "release", rel.Name, "namespace", rel.Namespace)
	// this will confirm the values set during installation
	log.Info("chart values", "config", rel.Config)

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
