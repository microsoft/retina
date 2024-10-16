package templates

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	kapingerPort       = int32(8080)
	KapingerDeployment = appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "template-deployment",
			Labels: map[string]string{
				"app":     "kapinger",
				"is-real": "true",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":     "kapinger",
					"is-real": "true",
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":     "kapinger",
						"is-real": "true",
					},
				},
				Spec: v1.PodSpec{
					NodeSelector: map[string]string{
						"kubernetes.io/arch": "amd64",
						"kubernetes.io/os":   "linux",
						"scale-test":         "true",
					},
					Containers: []v1.Container{
						{
							Name:  "container",
							Image: "acnpublic.azurecr.io/matmerr-pinger:v32",
							Resources: v1.ResourceRequirements{
								Limits: v1.ResourceList{
									"memory": resource.MustParse("20Mi"),
								},
								Requests: v1.ResourceList{
									"memory": resource.MustParse("20Mi"),
								},
							},
							Ports: []v1.ContainerPort{
								{
									ContainerPort: kapingerPort,
								},
							},
							Env: []v1.EnvVar{
								{
									Name:  "TARGET_TYPE",
									Value: "service",
								},
								{
									Name: "POD_IP",
									ValueFrom: &v1.EnvVarSource{
										FieldRef: &v1.ObjectFieldSelector{
											FieldPath: "status.podIP",
										},
									},
								},
								{
									Name: "POD_NAME",
									ValueFrom: &v1.EnvVarSource{
										FieldRef: &v1.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
								{
									Name:  "HTTP_PORT",
									Value: fmt.Sprintf("%d", kapingerPort),
								},
								{
									Name:  "TCP_PORT",
									Value: "8084",
								},
								{
									Name:  "UDP_PORT",
									Value: "8085",
								},
							},
						},
					},
				},
			},
		},
	}
)
