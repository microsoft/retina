package kubernetes

import (
	"context"
	"fmt"
	"strconv"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var ErrLabelMissingFromPod = fmt.Errorf("label missing from pod")

const (
	AgnhostHTTPPort  = 80
	AgnhostArchAmd64 = "amd64"
	AgnhostArchArm64 = "arm64"
)

type CreateAgnhostStatefulSet struct {
	AgnhostName        string
	AgnhostNamespace   string
	ScheduleOnSameNode bool
	KubeConfigFilePath string
	AgnhostArch        string
	AgnhostReplicas    *int
}

func (c *CreateAgnhostStatefulSet) Run() error {
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

	// set default arch to amd64
	if c.AgnhostArch == "" {
		c.AgnhostArch = AgnhostArchAmd64
	}

	// set default replicas to 1
	replicas := 1
	if c.AgnhostReplicas != nil {
		replicas = *c.AgnhostReplicas
	}

	agnhostStatefulSet := c.getAgnhostDeployment(c.AgnhostArch, replicas)

	err = CreateResource(ctx, agnhostStatefulSet, clientset)
	if err != nil {
		return fmt.Errorf("error agnhost component: %w", err)
	}

	selector, exists := agnhostStatefulSet.Spec.Selector.MatchLabels["app"]
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

func (c *CreateAgnhostStatefulSet) Prevalidate() error {
	return nil
}

func (c *CreateAgnhostStatefulSet) Stop() error {
	return nil
}

func (c *CreateAgnhostStatefulSet) getAgnhostDeployment(arch string, replicas int) *appsv1.StatefulSet {
	if replicas < 1 {
		replicas = 1
	}
	reps := int32(replicas) //nolint:gosec // replicas controlled by test code

	var affinity *v1.Affinity
	if c.ScheduleOnSameNode {
		affinity = &v1.Affinity{
			PodAffinity: &v1.PodAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{
					{
						TopologyKey: "kubernetes.io/hostname",
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"k8s-app": "agnhost",
							},
						},
					},
				},
			},
		}
	} else {
		affinity = &v1.Affinity{
			PodAntiAffinity: &v1.PodAntiAffinity{
				// prefer an even spread across the cluster to avoid scheduling on the same node
				PreferredDuringSchedulingIgnoredDuringExecution: []v1.WeightedPodAffinityTerm{
					{
						Weight: MaxAffinityWeight,
						PodAffinityTerm: v1.PodAffinityTerm{
							TopologyKey: "kubernetes.io/hostname",
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"k8s-app": "agnhost",
								},
							},
						},
					},
				},
			},
		}
	}

	return &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.AgnhostName,
			Namespace: c.AgnhostNamespace,
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: c.AgnhostName,
			Replicas:    &reps,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":     c.AgnhostName,
					"k8s-app": "agnhost",
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":     c.AgnhostName,
						"k8s-app": "agnhost",
					},
				},

				Spec: v1.PodSpec{
					Affinity: affinity,
					NodeSelector: map[string]string{
						"kubernetes.io/os":   "linux",
						"kubernetes.io/arch": arch,
					},
					Containers: []v1.Container{
						{
							Name:  c.AgnhostName,
							Image: "registry.k8s.io/e2e-test-images/agnhost:2.40",
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
