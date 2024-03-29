// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package retinaendpointcontroller

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	errors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	retinav1alpha1 "github.com/microsoft/retina/crd/api/v1alpha1"
	"github.com/microsoft/retina/operator/cache"
	"github.com/microsoft/retina/pkg/log"
)

const (
	REQUEST_TIMEOUT = 15 * time.Second
	MAX_RETRIES     = 5
)

// RetinaEndpointReconciler managed the lifecycle of RetinaEndpoints from Pods.
type RetinaEndpointReconciler struct {
	client.Client
	podchannel chan cache.PodCacheObject
	l          *log.ZapLogger
	rtmu       sync.RWMutex
	retries    map[types.NamespacedName]int
}

func init() {
	log.SetupZapLogger(&log.LogOpts{
		File: false,
	})
}

func New(client client.Client, podchannel chan cache.PodCacheObject) *RetinaEndpointReconciler {
	return &RetinaEndpointReconciler{
		Client:     client,
		podchannel: podchannel,
		retries:    make(map[types.NamespacedName]int),
		l:          log.Logger().Named(string("retina-endpoint-controller")),
	}
}

// NOTE(mainrerd): Chances are that pod cache channel lost pods events during controller manager restart, we need to
// have full-set reconciliation to make sure all RetinaEndpoints are reconciled to Pods.
// when a pod reaches here, it indicates that there is a metricsconfiguration (or tracesconfiguration if enabled) that references it,
// or doesn't and a RetinaEndpoint needs to be created or updated.
// This is a blocking function, and will wait on the pod channel until a pod is received.
func (r *RetinaEndpointReconciler) ReconcilePod(pctx context.Context) {
	r.l.Info("Start to reconcile Pods for RetinaEndpoints")
	for {
		select {
		case pod := <-r.podchannel:
			if pod.Pod != nil && pod.Pod.Spec.HostNetwork {
				r.l.Debug("pod is host networked, skipping", zap.String("name", pod.Key.String()))
				continue
			}
			r.l.Debug("reconcileRetinaEndpointFromPod", zap.String("name", pod.Key.String()))
			ctx, cancel := context.WithTimeout(pctx, REQUEST_TIMEOUT)
			err := r.reconcileRetinaEndpointFromPod(ctx, pod)
			if err != nil {
				r.l.Error("error creating retina endpoint", zap.Error(err), zap.String("name", pod.Key.String()))
				r.l.Info("requeuing pod", zap.String("name", pod.Key.String()))
				r.requeuePodToRetinaEndpoint(ctx, pod)
			}
			cancel()
		case <-pctx.Done():
			r.l.Info("Stop reconciling Pods for RetinaEndpoints")
			return
		}
	}
}

// requeuePodToRetinaEndpoint is called when a pod is received from the pod channel, and it will writeback to the
// pod channel if there is an error
func (r *RetinaEndpointReconciler) requeuePodToRetinaEndpoint(ctx context.Context, pod cache.PodCacheObject) {
	r.rtmu.RLock()
	value, exists := r.retries[pod.Key]
	r.rtmu.RUnlock()
	if exists && value >= MAX_RETRIES {
		r.l.Error("max retries reached, not retrying", zap.String("name", pod.Key.String()))
		delete(r.retries, pod.Key)
		return
	}
	r.rtmu.Lock()
	r.retries[pod.Key]++
	value = r.retries[pod.Key]
	r.rtmu.Unlock()
	select {
	case r.podchannel <- pod:
		r.l.Debug(fmt.Sprintf("starting retry %d", value), zap.String("name", pod.Key.String()))
	case <-ctx.Done():
		r.l.Error(ctx.Err().Error(), zap.String("name", pod.Key.String()))
	}
}

// reconcileRetinaEndpointFromPod create/update or delete a RetinaEndpoint based on the existence of its corresponding Pod.
func (r *RetinaEndpointReconciler) reconcileRetinaEndpointFromPod(ctx context.Context, pod cache.PodCacheObject) error {
	// Delete the RetinaEndpoint if the Pod from the cache is nil.
	// it means that the pod either has been deleted, or no longer matches a configuration,
	if pod.Pod == nil {
		// first check if a retina endpoint exists with the same key
		got := &retinav1alpha1.RetinaEndpoint{}
		err := r.Client.Get(ctx, pod.Key, got, &client.GetOptions{})
		if err != nil && !errors.IsNotFound(err) {
			r.l.Error("Failed to get RetinaEndpoint from the Pod", zap.Error(err), zap.String("name", pod.Key.String()))
			return err
		} else if errors.IsNotFound(err) {
			// we don't want a retina endpoint to exist if there is no pod, and in this case there is no pod,
			// so we don't need to do anything
			r.l.Debug("RetinaEndpoint is not found, no delete is required", zap.String("name", pod.Key.String()))
			return nil
		} else {
			r.l.Info("deleting RetinaEndpoint", zap.String("name", pod.Key.String()))
			// if there is a retina endpoint, then we need to delete it
			err = r.Client.Delete(ctx, got)
			if err != nil {
				r.l.Error("error deleting RetinaEndpoint", zap.Error(err), zap.String("name", pod.Key.String()))
				return err
			}
		}
		return nil
	}

	// Create Or Update the RetinaEndpoint from a running Pod.

	if pod.Pod.Status.Phase != corev1.PodRunning {
		r.l.Debug("pod is not running, skipping", zap.String("name", pod.Key.String()), zap.String("pod phase", string(pod.Pod.Status.Phase)))
		return nil
	}

	containers := []retinav1alpha1.RetinaEndpointStatusContainers{}
	for _, container := range pod.Pod.Status.ContainerStatuses {
		containers = append(containers, retinav1alpha1.RetinaEndpointStatusContainers{
			Name: container.Name,
			ID:   container.ContainerID,
		})
	}

	ips := []string{}
	for _, ip := range pod.Pod.Status.PodIPs {
		ips = append(ips, ip.IP)
	}

	refs := []retinav1alpha1.OwnerReference{}
	for _, ref := range pod.Pod.OwnerReferences {
		refs = append(refs, retinav1alpha1.OwnerReference{
			APIVersion: ref.APIVersion,
			Kind:       ref.Kind,
			Name:       ref.Name,
		})
	}

	// check to see if a RetinaEndpoint exists with the same key
	got := &retinav1alpha1.RetinaEndpoint{}
	err := r.Client.Get(ctx, pod.Key, got, &client.GetOptions{})

	// if there is an error getting the retina endpoint, and the error is not that the retina endpoint can't be found
	if err != nil && !errors.IsNotFound(err) {
		r.l.Error("Failed to get RetinaEndpoint from the Pod", zap.Error(err), zap.String("name", pod.Key.String()))
		return err
	}

	if errors.IsNotFound(err) {
		if pod.Pod.Status.PodIP == "" || pod.Pod.Status.HostIP == "" {
			r.l.Debug("pod has no podIP or hostIP, skipping", zap.String("name", pod.Key.String()))
			return nil
		}

		// since the retina endpoint doesn't exist, then we need to create it
		new := &retinav1alpha1.RetinaEndpoint{
			ObjectMeta: v1.ObjectMeta{
				Name:      pod.Key.Name,
				Namespace: pod.Key.Namespace,
			},
			TypeMeta: v1.TypeMeta{
				Kind:       "RetinaEndpoint",
				APIVersion: "v1alpha1",
			},
			Spec: retinav1alpha1.RetinaEndpointSpec{
				PodIP:           pod.Pod.Status.PodIP,
				PodIPs:          ips,
				Containers:      containers,
				NodeIP:          pod.Pod.Status.HostIP,
				Labels:          pod.Pod.Labels,
				Annotations:     pod.Pod.Annotations,
				OwnerReferences: refs,
			},
		}

		r.l.Info("creating RetinaEndpoint", zap.String("name", pod.Key.String()))
		if err := r.Client.Create(ctx, new); err != nil {
			r.l.Error(err.Error(), zap.String("name", pod.Key.String()))
			return err
		}
		return nil
	}

	// if the retina endpoint exists, then we need to update it, but only on running pods.
	// If terminating or pending, we don't care. On delete we'll recnil pods, and we'll delete the retina endpoint
	// by creating a deep copy of the retinaendpoint we got, and patching the spec
	new := got.DeepCopy()
	new.Spec = retinav1alpha1.RetinaEndpointSpec{
		PodIP:           pod.Pod.Status.PodIP,
		PodIPs:          ips,
		Containers:      containers,
		NodeIP:          pod.Pod.Status.HostIP,
		Labels:          pod.Pod.Labels,
		Annotations:     pod.Pod.Annotations,
		OwnerReferences: refs,
	}

	r.l.Info("updating RetinaEndpoint", zap.String("name", pod.Key.String()))
	if err = r.Client.Patch(ctx, new, client.MergeFrom(got)); err != nil {
		r.l.Error("failed to patch RetinaEndpoint", zap.Error(err), zap.String("name", pod.Key.String()))
		return err
	}
	return nil
}
