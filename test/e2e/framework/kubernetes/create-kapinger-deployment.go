package kubernetes

import (
	"context"
	"fmt"
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	KapingerHTTPPort  = 8080
	KapingerTCPPort   = 8085
	KapingerUDPPort   = 8086
	MaxAffinityWeight = 100
)

type CreateKapingerDeployment struct {
	KapingerNamespace  string
	KapingerReplicas   string
	KubeConfigFilePath string
}

func (c *CreateKapingerDeployment) Run() error {
	_, err := strconv.Atoi(c.KapingerReplicas)
	if err != nil {
		return fmt.Errorf("error converting replicas to int for Kapinger replicas: %w", err)
	}

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

	resources := []runtime.Object{
		c.GetKapingerService(),
		c.GetKapingerServiceAccount(),
		c.GetKapingerClusterRole(),
		c.GetKapingerClusterRoleBinding(),
		c.GetKapingerDeployment(),
	}

	for i := range resources {
		err = CreateResource(ctx, resources[i], clientset)
		if err != nil {
			return fmt.Errorf("error kapinger component: %w", err)
		}
	}

	return nil
}

func (c *CreateKapingerDeployment) Prevalidate() error {
	return nil
}

func (c *CreateKapingerDeployment) Stop() error {
	return nil
}

func (c *CreateKapingerDeployment) GetKapingerDeployment() *appsv1.Deployment {
	replicas, err := strconv.ParseInt(c.KapingerReplicas, 10, 32)
	if err != nil {
		fmt.Println("Error converting replicas to int for Kapinger replicas: ", err)
		return nil
	}
	reps := int32(replicas)

	return &appsv1.Deployment{
		TypeMeta: metaV1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metaV1.ObjectMeta{
			Name:      "kapinger",
			Namespace: c.KapingerNamespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &reps,
			Selector: &metaV1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "kapinger",
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metaV1.ObjectMeta{
					Labels: map[string]string{
						"app":    "kapinger",
						"server": "good",
					},
				},

				Spec: v1.PodSpec{
					NodeSelector: map[string]string{
						"kubernetes.io/os": "linux",
					},
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
												"app": "kapinger",
											},
										},
									},
								},
							},
						},
					},
					ServiceAccountName: "kapinger-sa",
					Containers: []v1.Container{
						{
							Name:  "kapinger",
							Image: "acnpublic.azurecr.io/kapinger:v0.0.23-9-g23ef222",
							Resources: v1.ResourceRequirements{
								Requests: v1.ResourceList{
									"memory": resource.MustParse("20Mi"),
								},
								Limits: v1.ResourceList{
									"memory": resource.MustParse("100Mi"),
								},
							},
							Ports: []v1.ContainerPort{
								{
									ContainerPort: KapingerHTTPPort,
								},
							},
							Env: []v1.EnvVar{
								{
									Name:  "GODEBUG",
									Value: "netdns=go",
								},
								{
									Name:  "TARGET_TYPE",
									Value: "service",
								},
								{
									Name:  "HTTP_PORT",
									Value: strconv.Itoa(KapingerHTTPPort),
								},
								{
									Name:  "TCP_PORT",
									Value: strconv.Itoa(KapingerTCPPort),
								},
								{
									Name:  "UDP_PORT",
									Value: strconv.Itoa(KapingerUDPPort),
								},
							},
						},
					},
				},
			},
		},
	}
}

func (c *CreateKapingerDeployment) GetKapingerService() *v1.Service {
	return &v1.Service{
		TypeMeta: metaV1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metaV1.ObjectMeta{
			Name:      "kapinger-service",
			Namespace: c.KapingerNamespace,
			Labels: map[string]string{
				"app": "kapinger",
			},
		},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{
				"app": "kapinger",
			},
			Ports: []v1.ServicePort{
				{
					Port:       KapingerHTTPPort,
					Protocol:   v1.ProtocolTCP,
					TargetPort: intstr.FromInt(KapingerHTTPPort),
				},
			},
		},
	}
}

func (c *CreateKapingerDeployment) GetKapingerServiceAccount() *v1.ServiceAccount {
	return &v1.ServiceAccount{
		TypeMeta: metaV1.TypeMeta{
			Kind:       "ServiceAccount",
			APIVersion: "v1",
		},
		ObjectMeta: metaV1.ObjectMeta{
			Name:      "kapinger-sa",
			Namespace: c.KapingerNamespace,
		},
	}
}

func (c *CreateKapingerDeployment) GetKapingerClusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		TypeMeta: metaV1.TypeMeta{
			Kind:       "ClusterRole",
			APIVersion: "rbac.authorization.k8s.io/v1",
		},
		ObjectMeta: metaV1.ObjectMeta{
			Name:      "kapinger-role",
			Namespace: c.KapingerNamespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"services", "pods"},
				Verbs:     []string{"get", "list"},
			},
		},
	}
}

func (c *CreateKapingerDeployment) GetKapingerClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		TypeMeta: metaV1.TypeMeta{
			Kind:       "ClusterRoleBinding",
			APIVersion: "rbac.authorization.k8s.io/v1",
		},
		ObjectMeta: metaV1.ObjectMeta{
			Name:      "kapinger-rolebinding",
			Namespace: c.KapingerNamespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "kapinger-sa",
				Namespace: c.KapingerNamespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "kapinger-role",
		},
	}
}
