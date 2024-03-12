/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package podcontroller

import (
	"context"
	"reflect"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controller "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	retinav1alpha1 "github.com/microsoft/retina/crd/api/v1alpha1"
	"github.com/microsoft/retina/operator/cache"
	"github.com/microsoft/retina/pkg/log"
)

// arbitrary selection, but no issues so far
const MAX_RECONCILERS = 5

// PodReconciler reconciles a pod object
type PodReconciler struct {
	client.Client
	Scheme     *k8sruntime.Scheme
	podchannel chan<- cache.PodCacheObject
	l          *log.ZapLogger
}

func init() {
	log.SetupZapLogger(&log.LogOpts{
		File: false,
	})
}

func New(client client.Client, scheme *k8sruntime.Scheme, podchannel chan<- cache.PodCacheObject) *PodReconciler {
	return &PodReconciler{
		l:          log.Logger().Named(string("pod-controller")),
		Client:     client,
		Scheme:     scheme,
		podchannel: podchannel,
	}
}

//+kubebuilder:rbac:groups=operator.retina.sh,resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=operator.retina.sh,resources=pods/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=operator.retina.sh,resources=pods/finalizers,verbs=update

func (r *PodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	pod := &corev1.Pod{}
	if err := r.Client.Get(ctx, req.NamespacedName, pod); err != nil {
		if apierrors.IsNotFound(err) {

			// Object not found, it has probably been deleted, forward to pod channel with nil pod
			r.l.Debug("pod deleted", zap.String("name", req.NamespacedName.String()))
			r.podchannel <- cache.PodCacheObject{Key: req.NamespacedName, Pod: nil}
			return ctrl.Result{}, nil
		}

		// Error getting object, return error
		return ctrl.Result{}, err
	}

	r.podchannel <- cache.PodCacheObject{Key: req.NamespacedName, Pod: pod}
	r.l.Debug("pod reconciled", zap.String("name", req.NamespacedName.String()))

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		WithEventFilter(onlyRunningOrDeletedPredicate()).
		WithOptions(controller.Options{
			// TODO: Remove comment when CR is upgraded.
			// MaxConcurrentReconciles: MAX_RECONCILERS,
		}).
		Complete(r)
}

// Ignore noisy pod create pod events, only filter if created event and pod is running
func filterPodCreateEvents(objMeta metav1.Object) bool {
	if pod, ok := objMeta.(*v1.Pod); ok {
		if pod.Spec.HostNetwork {
			return false
		}

		if pod.Status.Phase == v1.PodRunning || pod.DeletionTimestamp != nil {
			return true
		}

		if pod.Status.PodIP != "" {
			return false
		}
	}
	return false
}

// Ignore noisy pod update events, only filter if pod ip, pod ips, pod labels, or owner references update
func filterPodUpdateEvents(old metav1.Object, new metav1.Object) bool {
	oldpod, oldok := old.(*v1.Pod)
	newpod, newok := new.(*v1.Pod)

	// we only care if update is for pod ip or pod ips, and pod labels update
	if oldok && newok {
		if newpod.Spec.HostNetwork {
			return false
		}

		if newpod.Status.PodIP != oldpod.Status.PodIP {
			return true
		}

		if !reflect.DeepEqual(newpod.Status.PodIPs, oldpod.Status.PodIPs) {
			return true
		}

		if !reflect.DeepEqual(newpod.OwnerReferences, oldpod.OwnerReferences) {
			return true
		}

		if !reflect.DeepEqual(newpod.Labels, oldpod.Labels) {
			return true
		}

		if !reflect.DeepEqual(newpod.Annotations, oldpod.Annotations) {
			return true
		}

		if newpod.Status.PodIP != "" && len(newpod.Status.PodIPs) > 0 && !reflect.DeepEqual(getContainers(oldpod), getContainers(newpod)) {
			return true
		}
	}
	return false
}

func getContainers(pod *v1.Pod) []retinav1alpha1.RetinaEndpointStatusContainers {
	containers := []retinav1alpha1.RetinaEndpointStatusContainers{}
	for _, container := range pod.Status.ContainerStatuses {
		containers = append(containers, retinav1alpha1.RetinaEndpointStatusContainers{
			Name: container.Name,
			ID:   container.ContainerID,
		})
	}
	return containers
}

func onlyRunningOrDeletedPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return filterPodCreateEvents(e.Object)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return filterPodUpdateEvents(e.ObjectOld, e.ObjectNew)
		},
	}
}
