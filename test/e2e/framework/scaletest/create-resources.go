package scaletest

import (
	"context"
	"fmt"
	"log"

	e2ekubernetes "github.com/microsoft/retina/test/e2e/framework/kubernetes"
	"github.com/microsoft/retina/test/retry"
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
	NumLinuxDeployments          int
	NumWindowsDeployments        int
	NumRealReplicas              int
	NumLinuxServices             int
	NumWindowsServices           int
	NumUniqueLabelsPerDeployment int
}

// Useful when wanting to do parameter checking, for example
// if a parameter length is known to be required less than 80 characters,
// do this here so we don't find out later on when we run the step
// when possible, try to avoid making external calls, this should be fast and simple
func (c *CreateResources) Prevalidate() error {
	err := validateDeploymentType(c.RealPodType)
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

	ctx := context.TODO()

	retrier := retry.Retrier{Attempts: defaultRetryAttempts, Delay: defaultRetryDelay}

	for _, resource := range resources {
		err := retrier.Do(ctx, func() error {
			return e2ekubernetes.CreateResource(ctx, resource, clientset)
		})
		if err != nil {
			return fmt.Errorf("error creating resource: %w", err)
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

	kapinger := e2ekubernetes.CreateKapingerDeployment{
		KapingerNamespace:  c.Namespace,
		KubeConfigFilePath: c.KubeConfigFilePath,
	}
	kapingerClusterRole := kapinger.GetKapingerClusterRole()

	kapingerClusterRoleBinding := kapinger.GetKapingerClusterRoleBinding()

	kapingerSA := kapinger.GetKapingerServiceAccount()

	objs = append(objs, kapingerClusterRole, kapingerClusterRoleBinding, kapingerSA)

	services := c.generateServices("linux", c.NumLinuxServices)
	services = append(services, c.generateServices("windows", c.NumWindowsServices)...)
	objs = append(objs, services...)

	realDeployments := c.generateDeployments("linux", c.NumLinuxDeployments)
	realDeployments = append(realDeployments, c.generateDeployments("windows", c.NumWindowsDeployments)...)
	objs = append(objs, realDeployments...)

	// c.generateKwokNodes()
	log.Println("Finished generating YAMLs")
	return objs
}

func (c *CreateResources) generateDeployments(os string, count int) []runtime.Object {
	objs := []runtime.Object{}

	kapinger := e2ekubernetes.CreateKapingerDeployment{
		KapingerNamespace:  c.Namespace,
		KapingerReplicas:   fmt.Sprintf("%d", c.NumRealReplicas),
		KubeConfigFilePath: c.KubeConfigFilePath,
	}
	template := kapinger.GetKapingerDeployment()

	if template.Labels == nil {
		template.Labels = make(map[string]string)
	}
	template.Labels["is-real"] = "true"
	template.Labels[TrafficOSLabel] = os
	template.Spec.Template.Labels["is-real"] = "true"
	template.Spec.Template.Labels[TrafficOSLabel] = os
	template.Spec.Template.Spec.NodeSelector["scale-test"] = "true"
	template.Spec.Template.Spec.NodeSelector["kubernetes.io/arch"] = "amd64"
	template.Spec.Template.Spec.NodeSelector["kubernetes.io/os"] = os

	for i := 0; i < count; i++ {
		deployment := template.DeepCopy()

		name := fmt.Sprintf("%s-%s-dep-%05d", c.RealPodType, os, i)
		labelPrefix := fmt.Sprintf("%s-dep-lab", name)

		deployment.Name = name
		deployment.Labels["name"] = name
		deployment.Spec.Template.Labels["name"] = name

		r := int32(c.NumRealReplicas)
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

func (c *CreateResources) generateServices(os string, count int) []runtime.Object {
	objs := []runtime.Object{}

	kapingerSvc := e2ekubernetes.CreateKapingerDeployment{
		KapingerNamespace:  c.Namespace,
		KubeConfigFilePath: c.KubeConfigFilePath,
	}

	for i := 0; i < count; i++ {
		template := kapingerSvc.GetKapingerService()

		name := fmt.Sprintf("%s-%s-svc-%05d", c.RealPodType, os, i)
		template.Name = name
		template.Labels[TrafficOSLabel] = os

		template.Spec.Selector["name"] = fmt.Sprintf("%s-%s-dep-%05d", c.RealPodType, os, i)

		objs = append(objs, template)
	}
	return objs
}

func validateDeploymentType(depKind string) error {
	switch depKind {
	// case "kwok":
	// return templates.KwokDeployment.DeepCopy(), nil
	// case "real":
	// return templates.RealDeployment.DeepCopy(), nil
	case "kapinger":
		return nil
	default:
		return fmt.Errorf("invalid deployment kind: %s", depKind)
	}
}
