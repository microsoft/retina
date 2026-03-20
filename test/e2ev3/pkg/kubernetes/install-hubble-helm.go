package kubernetes

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	e2ecfg "github.com/microsoft/retina/test/e2ev3/config"
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
	ImageTag           string
	ImageRegistry      string
	ImageNamespace     string
	HelmDriver         string
	ImageLoader        e2ecfg.ClusterProvider
}

func (v *InstallHubbleHelmChart) String() string { return "install-hubble-helm" }

func (v *InstallHubbleHelmChart) Do(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, defaultTimeoutSeconds*time.Second)
	defer cancel()

	settings := cli.New()
	settings.KubeConfig = v.KubeConfigFilePath
	actionConfig := new(action.Configuration)

	err := actionConfig.Init(settings.RESTClientGetter(), v.Namespace, v.HelmDriver, func(format string, v ...any) { slog.Info(fmt.Sprintf(format, v...)) })
	if err != nil {
		return fmt.Errorf("failed to initialize helm action config: %w", err)
	}

	// Creating extra namespace to deploy test pods
	rc, err := clientcmd.BuildConfigFromFlags("", v.KubeConfigFilePath)
	if err != nil {
		return fmt.Errorf("failed to build rest config: %w", err)
	}
	err = CreateNamespaceFn(ctx, rc, e2ecfg.TestPodNamespace)
	if err != nil {
		return fmt.Errorf("failed to create namespace %s: %w", v.Namespace, err)
	}

	tag := v.ImageTag
	if tag == "" {
		return fmt.Errorf("tag is not set: %w", errEmpty)
	}
	imageRegistry := v.ImageRegistry
	if imageRegistry == "" {
		return fmt.Errorf("image registry is not set: %w", errEmpty)
	}

	imageNamespace := v.ImageNamespace
	if imageNamespace == "" {
		return fmt.Errorf("image namespace is not set: %w", errEmpty)
	}

	// load chart from the path
	chart, err := loader.Load(v.ChartPath)
	if err != nil {
		return fmt.Errorf("failed to load chart from path %s: %w", v.ChartPath, err)
	}

	if secrets := v.ImageLoader.ImagePullSecrets(); len(secrets) > 0 {
		chart.Values["imagePullSecrets"] = secrets
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
		slog.Info("found existing release, removing before installing", "release", release.Name)
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

	slog.Info("installed chart", "release", rel.Name, "namespace", rel.Namespace)
	slog.Info("chart values", "config", rel.Config)

	// ensure all pods are running, since helm doesn't care about windows
	config, err := clientcmd.BuildConfigFromFlags("", v.KubeConfigFilePath)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("error creating Kubernetes client: %w", err)
	}

	// Validate Hubble Relay and UI pods in parallel.
	var relayErr, uiErr error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		relayErr = WaitForPodReady(ctx, clientset, HubbleNamespace, "k8s-app="+HubbleRelayApp)
	}()
	go func() {
		defer wg.Done()
		uiErr = WaitForPodReady(ctx, clientset, HubbleNamespace, "k8s-app="+HubbleUIApp)
	}()
	wg.Wait()

	if relayErr != nil {
		return fmt.Errorf("error waiting for Hubble Relay pods to be ready: %w", relayErr)
	}
	slog.Info("Hubble Relay pod is ready")

	if uiErr != nil {
		return fmt.Errorf("error waiting for Hubble UI pods to be ready: %w", uiErr)
	}
	slog.Info("Hubble UI pod is ready")

	return nil
}
