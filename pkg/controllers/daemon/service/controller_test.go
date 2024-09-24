package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

func TestServiceFilter(t *testing.T) {
	pred := predicate.NewPredicateFuncs(func(object client.Object) bool {
		service, ok := object.(*corev1.Service)
		if !ok {
			return false
		}
		return service.Spec.ClusterIP != "None"
	})

	tests := []struct {
		name   string
		event  interface{}
		expect bool
	}{
		{
			name: "CreateEvent with ClusterIP None",
			event: event.CreateEvent{
				Object: &corev1.Service{
					Spec: corev1.ServiceSpec{
						ClusterIP: "None",
					},
				},
			},
			expect: false,
		},
		{
			name: "CreateEvent with ClusterIP not None",
			event: event.CreateEvent{
				Object: &corev1.Service{
					Spec: corev1.ServiceSpec{
						ClusterIP: "10.0.0.1",
					},
				},
			},
			expect: true,
		},
		{
			name: "UpdateEvent with ClusterIP None",
			event: event.UpdateEvent{
				ObjectNew: &corev1.Service{
					Spec: corev1.ServiceSpec{
						ClusterIP: "None",
					},
				},
			},
			expect: false,
		},
		{
			name: "UpdateEvent with ClusterIP not None",
			event: event.UpdateEvent{
				ObjectNew: &corev1.Service{
					Spec: corev1.ServiceSpec{
						ClusterIP: "10.0.0.1",
					},
				},
			},
			expect: true,
		},
		{
			name: "DeleteEvent with ClusterIP None",
			event: event.DeleteEvent{
				Object: &corev1.Service{
					Spec: corev1.ServiceSpec{
						ClusterIP: "None",
					},
				},
			},
			expect: false,
		},
		{
			name: "DeleteEvent with ClusterIP not None",
			event: event.DeleteEvent{
				Object: &corev1.Service{
					Spec: corev1.ServiceSpec{
						ClusterIP: "10.0.0.1",
					},
				},
			},
			expect: true,
		},
		{
			name: "GenericEvent with ClusterIP None",
			event: event.GenericEvent{
				Object: &corev1.Service{
					Spec: corev1.ServiceSpec{
						ClusterIP: "None",
					},
				},
			},
			expect: false,
		},
		{
			name: "GenericEvent with ClusterIP not None",
			event: event.GenericEvent{
				Object: &corev1.Service{
					Spec: corev1.ServiceSpec{
						ClusterIP: "10.0.0.1",
					},
				},
			},
			expect: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result bool
			switch e := tt.event.(type) {
			case event.CreateEvent:
				result = pred.Create(e)
			case event.UpdateEvent:
				result = pred.Update(e)
			case event.DeleteEvent:
				result = pred.Delete(e)
			case event.GenericEvent:
				result = pred.Generic(e)
			}
			assert.Equal(t, tt.expect, result)
		})
	}
}
