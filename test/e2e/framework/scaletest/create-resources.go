package scaletest

import (
	"context"
	"fmt"
	"log"
	"time"

	e2ekubernetes "github.com/microsoft/retina/test/e2e/framework/kubernetes"
	"github.com/microsoft/retina/test/e2e/framework/scaletest/templates"
	yaml "gopkg.in/yaml.v2"
	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type CreateResources struct {
	Namespace                    string
	KubeConfigFilePath           string
	NumKwokDeployments           int
	NumKwokReplicas              int
	RealPodType                  string
	NumRealDeployments           int
	NumRealReplicas              int
	NumRealServices              int
	NumUniqueLabelsPerDeployment int
	DryRun                       bool
}

// Useful when wanting to do parameter checking, for example
// if a parameter length is known to be required less than 80 characters,
// do this here so we don't find out later on when we run the step
// when possible, try to avoid making external calls, this should be fast and simple
func (c *CreateResources) Prevalidate() error {
	_, err := getDeploymentTemplate(c.RealPodType)
	return err
}

// Primary step where test logic is executed
// Returning an error will cause the test to fail
func (c *CreateResources) Run() error {
	resources := c.getResources()

	config, err := clientcmd.BuildConfigFromFlags("", c.KubeConfigFilePath)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("error creating Kubernetes client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeoutSeconds*time.Second)
	defer cancel()

	for _, resource := range resources {
		if c.DryRun {
			y, err := yaml.Marshal(resource)
			if err != nil {
				return fmt.Errorf("error marshalling resource: %w", err)
			}
			log.Printf("%s", string(y))
		} else {
			e2ekubernetes.CreateResource(ctx, resource, clientset)
		}
	}

	return nil
}

// Require for background steps
func (c *CreateResources) Stop() error {
	return nil
}

func (c *CreateResources) getResources() []runtime.Object {
	log.Println("Generating Resources")

	objs := []runtime.Object{}

	// kwokDeployments := c.generateDeployments(c.NumKwokDeployments, c.NumKwokReplicas, "kwok")
	// objs = append(objs, kwokDeployments...)

	realDeployments := c.generateDeployments(c.NumRealDeployments, c.NumRealReplicas, c.RealPodType)
	objs = append(objs, realDeployments...)

	services := c.generateServices(c.NumRealServices, "real", c.RealPodType)
	objs = append(objs, services...)

	kapingerClusterRole := templates.KapingerClusterRole.DeepCopy()
	kapingerClusterRole.Namespace = c.Namespace

	kapingerClusterRoleBinding := templates.KapingerClusterRoleBinding.DeepCopy()
	kapingerClusterRoleBinding.Namespace = c.Namespace
	kapingerClusterRoleBinding.Subjects[0].Namespace = c.Namespace

	objs = append(objs, kapingerClusterRole, kapingerClusterRoleBinding)
	// c.generateKwokNodes()
	log.Println("Finished generating YAMLs")
	return objs
}

func (c *CreateResources) generateDeployments(numDeployments, numReplicas int, depKind string) []runtime.Object {
	objs := []runtime.Object{}
	template, _ := getDeploymentTemplate(depKind)

	for i := 0; i < numDeployments; i++ {
		deployment := template.DeepCopy()

		name := fmt.Sprintf("%s-dep-%05d", depKind, i)
		labelPrefix := fmt.Sprintf("%s-dep-lab", name)

		deployment.Name = name
		deployment.Namespace = c.Namespace

		r := int32(numReplicas)
		deployment.Spec.Replicas = &r

		for j := 0; j < c.NumUniqueLabelsPerDeployment; j++ {
			labelKey := fmt.Sprintf("%s-%05d", labelPrefix, j)
			labelVal := "val"
			deployment.Spec.Selector.MatchLabels[labelKey] = labelVal
			deployment.Spec.Template.Labels[labelKey] = labelVal
		}

		objs = append(objs, deployment)
	}

	return objs
}

func (c *CreateResources) generateServices(numServices int, svcKind, depKind string) []runtime.Object {
	objs := []runtime.Object{}

	for i := 0; i < numServices; i++ {
		name := fmt.Sprintf("%s-svc-%05d", svcKind, i)

		template := templates.Service.DeepCopy()
		template.Name = name

		template.Namespace = c.Namespace

		template.Spec.Selector["name"] = fmt.Sprintf("%s-%s-dep-%05d", svcKind, depKind, i)

		objs = append(objs, template)
	}
	return objs
}

func getDeploymentTemplate(depKind string) (*v1.Deployment, error) {
	switch depKind {
	// case "kwok":
	// return templates.KwokDeployment.DeepCopy(), nil
	// case "real":
	// return templates.RealDeployment.DeepCopy(), nil
	case "kapinger":
		return templates.KapingerDeployment.DeepCopy(), nil
	default:
		return nil, fmt.Errorf("invalid deployment kind: %s", depKind)
	}
}
