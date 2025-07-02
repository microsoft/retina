package kubernetes

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	applyTimeout = 10 * time.Minute
)

type ApplyYamlConfig struct {
	KubeConfigFilePath string
	YamlFilePath       string
}

func (a *ApplyYamlConfig) Run() error {
	ctx, cancel := context.WithTimeout(context.Background(), applyTimeout)
	defer cancel()

	config, err := clientcmd.BuildConfigFromFlags("", a.KubeConfigFilePath)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("error creating dynamic client: %w", err)
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return fmt.Errorf("error creating discovery client: %w", err)
	}

	resources, err := restmapper.GetAPIGroupResources(discoveryClient)
	if err != nil {
		return fmt.Errorf("error getting API group resources: %w", err)
	}

	mapper := restmapper.NewDiscoveryRESTMapper(resources)

	yamlFile, err := os.ReadFile(a.YamlFilePath)
	if err != nil {
		return fmt.Errorf("error reading YAML file: %w", err)
	}

	reader := bytes.NewReader(yamlFile)
	decoder := yaml.NewYAMLOrJSONDecoder(reader, 100)
	var rawObj unstructured.Unstructured
	if err := decoder.Decode(&rawObj); err != nil {
		return fmt.Errorf("error decoding YAML file: %w", err)
	}

	// Get GroupVersionResource to invoke the dynamic client
	gvk := rawObj.GroupVersionKind()
	restMapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return fmt.Errorf("error getting REST mapping: %w", err)
	}
	gvr := restMapping.Resource

	// Apply the YAML document
	namespace := rawObj.GetNamespace()
	if len(namespace) == 0 {
		namespace = "default"
	}
	applyOpts := metav1.ApplyOptions{FieldManager: "kube-apply"}
	_, err = dynamicClient.Resource(gvr).Namespace(namespace).Apply(ctx, rawObj.GetName(), &rawObj, applyOpts)
	if err != nil {
		return fmt.Errorf("apply error: %w", err)
	}

	log.Printf("applied YAML file: %s\n", a.YamlFilePath)
	return nil
}

func (a *ApplyYamlConfig) Prevalidate() error {
	_, err := os.Stat(a.YamlFilePath)
	if os.IsNotExist(err) {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current working directory %s: %w", cwd, err)
		}
		log.Printf("the current working directory %s", cwd)
		return fmt.Errorf("YAML file not found at %s: working directory: %s: %w", a.YamlFilePath, cwd, err)
	}
	log.Printf("found YAML file at %s", a.YamlFilePath)

	return nil
}

func (a *ApplyYamlConfig) Stop() error {
	return nil
}
