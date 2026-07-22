package kubernetes

import (
	"context"
	"fmt"
	"strings"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	Egress  = "egress"
	Ingress = "ingress"
)

type CreateDenyAllNetworkPolicy struct {
	NetworkPolicyNamespace string
	KubeConfigFilePath     string
	DenyAllLabelSelector   string
}

func (c *CreateDenyAllNetworkPolicy) Run() error {
	config, err := clientcmd.BuildConfigFromFlags("", c.KubeConfigFilePath)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("error creating Kubernetes client: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	networkPolicy := getNetworkPolicy(c.NetworkPolicyNamespace, c.DenyAllLabelSelector)
	err = CreateResource(ctx, networkPolicy, clientset)
	if err != nil {
		return fmt.Errorf("error creating simple deny-all network policy: %w", err)
	}

	return nil
}

func getNetworkPolicy(namespace, labelSelector string) *networkingv1.NetworkPolicy {
	labelSelectorSlice := strings.Split(labelSelector, "=")
	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "deny-all",
			Namespace: namespace,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					labelSelectorSlice[0]: labelSelectorSlice[1],
				},
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress,
			},
			Egress:  []networkingv1.NetworkPolicyEgressRule{},
			Ingress: []networkingv1.NetworkPolicyIngressRule{},
		},
	}
}

func (c *CreateDenyAllNetworkPolicy) Prevalidate() error {
	return nil
}

func (c *CreateDenyAllNetworkPolicy) Stop() error {
	return nil
}

type DeleteDenyAllNetworkPolicy struct {
	NetworkPolicyNamespace string
	KubeConfigFilePath     string
	DenyAllLabelSelector   string
}

func (d *DeleteDenyAllNetworkPolicy) Run() error {
	config, err := clientcmd.BuildConfigFromFlags("", d.KubeConfigFilePath)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("error creating Kubernetes client: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	networkPolicy := getNetworkPolicy(d.NetworkPolicyNamespace, d.DenyAllLabelSelector)
	err = DeleteResource(ctx, networkPolicy, clientset)
	if err != nil {
		return fmt.Errorf("error creating simple deny-all network policy: %w", err)
	}

	return nil
}

func (d *DeleteDenyAllNetworkPolicy) Prevalidate() error {
	return nil
}
