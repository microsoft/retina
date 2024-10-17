package templates

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var (
	defaultPort       = int32(8080)
	defaultTargetPort = 8080
	Service           = v1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "template-service",
			Labels: map[string]string{
				"app":     "kapinger",
				"is-real": "true",
			},
		},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{
				"app":     "kapinger",
				"is-real": "true",
			},
			Ports: []v1.ServicePort{
				{
					Protocol:   v1.ProtocolTCP,
					Port:       defaultPort,
					TargetPort: intstr.FromInt(defaultTargetPort),
				},
			},
		},
	}
)
