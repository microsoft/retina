package kubernetes

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/microsoft/retina/test/e2e/framework/types"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var ErrLabelMissingFromPod = fmt.Errorf("label missing from pod")

const (
	AgnhostHTTPPort = 80
	AgnhostReplicas = 1
)

type CreateAgnhostStatefulSet struct {
	AgnhostName        string
	AgnhostNamespace   string
	KubeConfigFilePath string
}

func (c *CreateAgnhostStatefulSet) Run(_ *types.RuntimeObjects) error {
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

	agnhostStatefulest := c.getAgnhostDeployment()

	err = CreateResource(ctx, agnhostStatefulest, clientset)
	if err != nil {
		return fmt.Errorf("error agnhost component: %w", err)
	}

	selector, exists := agnhostStatefulest.Spec.Selector.MatchLabels["app"]
	if !exists {
		return fmt.Errorf("missing label \"app=%s\" from agnhost statefulset: %w", c.AgnhostName, ErrLabelMissingFromPod)
	}

	labelSelector := fmt.Sprintf("app=%s", selector)
	err = WaitForPodReady(ctx, clientset, c.AgnhostNamespace, labelSelector)
	if err != nil {
		return fmt.Errorf("error waiting for agnhost pod to be ready: %w", err)
	}

	return nil
}

func (c *CreateAgnhostStatefulSet) PreRun() error {
	return nil
}

func (c *CreateAgnhostStatefulSet) Stop() error {
	return nil
}

func (c *CreateAgnhostStatefulSet) getAgnhostDeployment() *appsv1.StatefulSet {
	reps := int32(AgnhostReplicas)

	return &appsv1.StatefulSet{
		TypeMeta: metaV1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metaV1.ObjectMeta{
			Name:      c.AgnhostName,
			Namespace: c.AgnhostNamespace,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &reps,
			Selector: &metaV1.LabelSelector{
				MatchLabels: map[string]string{
					"app":     c.AgnhostName,
					"k8s-app": "agnhost",
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metaV1.ObjectMeta{
					Labels: map[string]string{
						"app":     c.AgnhostName,
						"k8s-app": "agnhost",
					},
				},

				Spec: v1.PodSpec{
					Affinity: &v1.Affinity{
						PodAntiAffinity: &v1.PodAntiAffinity{
							// prefer an even spread across the cluster to avoid scheduling on the same node
							PreferredDuringSchedulingIgnoredDuringExecution: []v1.WeightedPodAffinityTerm{
								{
									Weight: MaxAffinityWeight,
									PodAffinityTerm: v1.PodAffinityTerm{
										TopologyKey: "kubernetes.io/hostname",
										LabelSelector: &metaV1.LabelSelector{
											MatchLabels: map[string]string{
												"k8s-app": "agnhost",
											},
										},
									},
								},
							},
						},
					},
					NodeSelector: map[string]string{
						"kubernetes.io/os": "linux",
					},
					Containers: []v1.Container{
						{
							Name:  c.AgnhostName,
							Image: "acnpublic.azurecr.io/agnhost:2.40",
							Resources: v1.ResourceRequirements{
								Requests: v1.ResourceList{
									"memory": resource.MustParse("20Mi"),
								},
								Limits: v1.ResourceList{
									"memory": resource.MustParse("20Mi"),
								},
							},
							Command: []string{
								"/agnhost",
							},
							Args: []string{
								"serve-hostname",
								"--http",
								"--port",
								strconv.Itoa(AgnhostHTTPPort),
							},

							Ports: []v1.ContainerPort{
								{
									ContainerPort: AgnhostHTTPPort,
								},
							},
							Env: []v1.EnvVar{},
						},
					},
				},
			},
		},
	}
}
