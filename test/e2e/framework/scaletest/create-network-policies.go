package scaletest

import (
	"context"
	"fmt"

	e2ekubernetes "github.com/microsoft/retina/test/e2e/framework/kubernetes"

	"github.com/microsoft/retina/test/e2e/framework/scaletest/templates"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type CreateNetworkPolicies struct {
	KubeConfigFilePath          string
	Namespace                   string
	NumNetworkPolicies          int
	NumUnappliedNetworkPolicies int
	NumSharedLabelsPerPod       int
}

// Useful when wanting to do parameter checking, for example
// if a parameter length is known to be required less than 80 characters,
// do this here so we don't find out later on when we run the step
// when possible, try to avoid making external calls, this should be fast and simple
func (c *CreateNetworkPolicies) Prevalidate() error {
	return nil
}

// Primary step where test logic is executed
// Returning an error will cause the test to fail
func (c *CreateNetworkPolicies) Run() error {
	config, err := clientcmd.BuildConfigFromFlags("", c.KubeConfigFilePath)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("error creating Kubernetes client: %w", err)
	}

	ctx := context.TODO()

	networkPolicies := c.generateNetworkPolicies(c.NumNetworkPolicies)

	for _, np := range networkPolicies {
		e2ekubernetes.CreateResource(ctx, np, clientset)
	}

	return nil
}

// Require for background steps
func (c *CreateNetworkPolicies) Stop() error {
	return nil
}

func (c *CreateNetworkPolicies) generateNetworkPolicies(numPolicies int) []runtime.Object {
	objs := []runtime.Object{}
	for i := 0; i < numPolicies; i++ {
		name := fmt.Sprintf("policy-%05d", i)

		template := templates.NetworkPolicy.DeepCopy()

		template.Name = name
		template.Namespace = c.Namespace

		valNum := i
		if valNum >= c.NumSharedLabelsPerPod-2 {
			valNum = c.NumSharedLabelsPerPod - 2
		}

		template.Spec.PodSelector.MatchLabels[fmt.Sprintf("shared-lab-%05d", valNum)] = "val"

		ingressNum := valNum + 1
		template.Spec.Ingress = []netv1.NetworkPolicyIngressRule{
			{
				From: []netv1.NetworkPolicyPeer{
					{
						PodSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								fmt.Sprintf("shared-lab-%05d", ingressNum): "val",
							},
						},
					},
				},
			},
		}

		egressNum := valNum + 2
		template.Spec.Egress = []netv1.NetworkPolicyEgressRule{
			{
				To: []netv1.NetworkPolicyPeer{
					{
						PodSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								fmt.Sprintf("shared-lab-%05d", egressNum): "val",
							},
						},
					},
				},
			},
		}

		objs = append(objs, template)
	}

	for i := 0; i < c.NumUnappliedNetworkPolicies; i++ {
		name := fmt.Sprintf("unapplied-policy-%05d", i)

		template := templates.NetworkPolicy.DeepCopy()

		template.Name = name
		template.Namespace = c.Namespace

		template.Spec.PodSelector.MatchLabels["non-existent-key"] = "val"

		template.Spec.Ingress = []netv1.NetworkPolicyIngressRule{
			{
				From: []netv1.NetworkPolicyPeer{
					{
						PodSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"non-existent-key": "val",
							},
						},
					},
				},
			},
		}

		template.Spec.Egress = []netv1.NetworkPolicyEgressRule{
			{
				To: []netv1.NetworkPolicyPeer{
					{
						PodSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"non-existent-key": "val",
							},
						},
					},
				},
			},
		}

		objs = append(objs, template)

	}

	return objs
}
