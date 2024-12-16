// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package endpointcontroller

import (
	"context"
	"encoding/json"
	"reflect"
	"sync"
	"time"

	"github.com/microsoft/retina/pkg/controllers/operator/cilium-crds/cache"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"go.uber.org/zap"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/cilium/cilium/pkg/hive/cell"
	"github.com/cilium/cilium/pkg/k8s"
	ciliumv2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	k8sClient "github.com/cilium/cilium/pkg/k8s/client"
	"github.com/cilium/cilium/pkg/k8s/client/clientset/versioned"
	"github.com/cilium/cilium/pkg/k8s/resource"
	slim_corev1 "github.com/cilium/cilium/pkg/k8s/slim/k8s/api/core/v1"
	slim_clientset "github.com/cilium/cilium/pkg/k8s/slim/k8s/client/clientset/versioned"
	"github.com/cilium/cilium/pkg/labels"
	"github.com/cilium/workerpool"
)

const (
	RequestTimeout = 15 * time.Second
	MaxWorkers     = 20

	// useOwnerReferences determines whether we set the ownerReferences field on CiliumEndpoints to the Pod that it is associated with.
	// With this, k8s will automatically delete the CiliumEndpoint when the Pod is deleted.
	// Not sure if this useful. Could be required for endpointgc.Cell?
	useOwnerReferences = false
)

var ErrClientsetDisabled = errors.New("failure due to clientset disabled")

// endpointReconciler managed the lifecycle of CiliumEndpoints and CiliumIdentities from Pods.
type endpointReconciler struct {
	*sync.Mutex
	l                   logrus.FieldLogger
	clientset           versioned.Interface
	ciliumSlimClientSet slim_clientset.Interface
	// podEvents represents Pod CRUD events relayed from the Pod controller. If the Pod was deleted, the PodCacheObject.Pod field will be nil
	podEvents chan cache.PodCacheObject
	// ciliumEndpoints has a store of CiliumEndpoints in API Server
	ciliumEndpoints resource.Resource[*ciliumv2.CiliumEndpoint]

	// pods is a store of Pods in API Server
	pods resource.Resource[*slim_corev1.Pod]

	// namespace is a store of Namespaces in API Server
	namespaces resource.Resource[*slim_corev1.Namespace]

	identityManager *IdentityManager

	// store of processed pods and namespaces
	// processedPodCache map in store pod key to PodEndpoint.
	// It contains only pods which we have processed via Pod events.
	// It contains endpoint goal state, and is independent of ciliumEndpoints store.
	// When endpointReconciler is leading, all endpoint state should be in API Server.
	store *Store

	wp *workerpool.WorkerPool
}

type params struct {
	cell.In

	Logger          logrus.FieldLogger
	Lifecycle       cell.Lifecycle
	Clientset       k8sClient.Clientset
	CiliumEndpoints resource.Resource[*ciliumv2.CiliumEndpoint]
	Namespaces      resource.Resource[*slim_corev1.Namespace]
	Pods            resource.Resource[*slim_corev1.Pod]
}

func registerEndpointController(p params) error {
	if !p.Clientset.IsEnabled() {
		return ErrClientsetDisabled
	}

	l := p.Logger.WithField("component", "endpointcontroller")
	r := &endpointReconciler{
		Mutex:               &sync.Mutex{},
		l:                   l,
		clientset:           p.Clientset,
		ciliumSlimClientSet: p.Clientset.Slim(),
		ciliumEndpoints:     p.CiliumEndpoints,
		pods:                p.Pods,
		namespaces:          p.Namespaces,
		store:               NewStore(),
	}

	p.Lifecycle.Append(r)

	return nil
}

func (r *endpointReconciler) Start(_ cell.HookContext) error {
	// NOTE: we must create IdentityManager on leader election since its allocator auto-starts on creation.
	// There is a way to disable auto-start but then there is no exposed function to simply start().
	im, err := NewIdentityManager(r.l, r.clientset)
	if err != nil {
		return errors.Wrap(err, "failed to create identity manager")
	}

	r.identityManager = im

	// making sure we have only one thread running at a time.
	r.wp = workerpool.New(MaxWorkers)

	if err := r.wp.Submit("namespace-controller", r.runNamespaceEvents); err != nil {
		return errors.Wrap(err, "failed to submit task to namespace workerpool")
	}

	if err := r.wp.Submit("endpoint-reconciler", r.run); err != nil {
		return errors.Wrap(err, "failed to submit task to endpoint workerpool")
	}

	return nil
}

func (r *endpointReconciler) Stop(_ cell.HookContext) error {
	if err := r.wp.Close(); err != nil {
		return errors.Wrap(err, "failed to stop endpoint workerpool")
	}

	return nil
}

func (r *endpointReconciler) run(pctx context.Context) error {
	r.l.Info("start to reconcile Pods for CiliumEndpoints")

	podEvents := r.pods.Events(pctx)

	for {
		select {
		case ev, ok := <-podEvents:
			if !ok {
				r.l.Info("Pod Events channel is closed. Stopping reconciling Pods for CiliumEndpoints")
				return nil
			}

			if ev.Object != nil && ev.Object.Spec.HostNetwork {
				r.l.WithField("podKey", ev.Key.String()).Debug("pod is host networked, skipping")
				ev.Done(nil)
				continue
			}

			err := r.wp.Submit("pod-event-handler", func(ctx context.Context) error {
				return r.runEventHandler(ctx, ev)
			})
			if err != nil {
				r.l.WithError(err).WithField("podKey", ev.Key.String()).Error("failed to submit pod event handler")
			}

		case <-pctx.Done():
			r.l.Info("stop reconciling Pods for CiliumEndpoints")
			return nil
		}
	}
}

func (r *endpointReconciler) runEventHandler(pctx context.Context, ev resource.Event[*slim_corev1.Pod]) error {
	var err error
	ctx, cancel := context.WithTimeout(pctx, RequestTimeout)
	switch ev.Kind {
	case resource.Sync:
		// Ignore the update/
	case resource.Upsert:
		// HANDLE UPSERT
		if ev.Object != nil && ev.Object.Spec.HostNetwork {
			r.l.WithField("podKey", ev.Key.String()).Debug("pod is host networked, skipping")
		} else {
			err = r.ReconcilePod(ctx, ev.Key, ev.Object)
		}
	case resource.Delete:
		err = r.HandlePodDelete(ctx, ev.Key)
	}
	cancel()

	if err != nil {
		r.l.WithError(err).WithField("podKey", ev.Key.String()).Error("error creating cilium endpoint. requeuing pod")
	}
	ev.Done(err)

	return nil
}

func (r *endpointReconciler) runNamespaceEvents(pctx context.Context) error {
	r.l.Info("start to reconcile Namespaces for CiliumEndpoints")

	namespaceEvents := r.namespaces.Events(pctx)

	for {
		select {
		case ev, ok := <-namespaceEvents:
			if !ok {
				r.l.Info("Namespace Events channel is closed. Stopping reconciling Namespaces for CiliumEndpoints")
				return nil
			}

			var err error
			ctx, cancel := context.WithTimeout(pctx, RequestTimeout)
			switch ev.Kind {
			case resource.Sync:
				// Ignore the update/
			case resource.Upsert:
				// HANDLE UPSERT
				err = r.reconcileNamespace(ctx, ev.Object)
			case resource.Delete:
				err = r.handleNamespaceDelete(ctx, ev.Key.Name)
			}
			cancel()

			if err != nil {
				r.l.WithError(err).WithField("namespaceKey", ev.Key.String()).Error("error creating cilium endpoint. requeuing namespace")
			}
			ev.Done(err)
		case <-pctx.Done():
			r.l.Info("stop reconciling Namespaces for CiliumEndpoints")
			return nil
		}
	}
}

func (r *endpointReconciler) ReconcilePodsInNamespace(ctx context.Context, namespace string) error {
	r.Mutex.Lock()
	defer r.Mutex.Unlock()
	r.l.Debug("reconciling pods in namespace", zap.String("namespace ", namespace))
	podList := r.store.ListPodKeysByNamespace(namespace)
	for _, podKey := range podList {
		pod, ok := r.store.GetPod(podKey)
		if !ok {
			r.l.WithField("podKey", podKey.String()).Debug("pod not found in cache, skipping")
			continue
		}

		if pod.toDelete {
			r.l.WithField("podKey", podKey.String()).Debug("pod marked for deletion, skipping")
			continue
		}

		newPEP := pod.deepCopy()
		endpointsLabels, err := r.ciliumEndpointsLabels(ctx, pod.podObj)
		if err != nil {
			return errors.Wrap(err, "failed to get pod labels")
		}
		newPEP.lbls = endpointsLabels

		r.l.Debug("upserting pod in namespace",
			zap.String("namespace ", namespace),
			zap.String("podKey", podKey.String()),
			zap.Any("old labels ", pod.lbls),
			zap.Any("new labels ", newPEP.lbls),
		)

		err = r.handlePodUpsert(ctx, newPEP)
		if err != nil {
			r.l.Error("failed to upsert pod", zap.Error(err), zap.String("podKey", podKey.String()))
		}

		if err != nil {
			return errors.Wrap(err, "failed to upsert pod")
		}

	}

	return nil
}

func (r *endpointReconciler) ReconcilePod(ctx context.Context, podKey resource.Key, pod *slim_corev1.Pod) error {
	r.Mutex.Lock()
	defer r.Mutex.Unlock()
	r.l.Debug("reconciling pod with lock", zap.String("namespace", podKey.Namespace), zap.String("pod ", podKey.Name))
	return r.reconcilePod(ctx, podKey, pod)
}

func (r *endpointReconciler) reconcilePod(ctx context.Context, podKey resource.Key, pod *slim_corev1.Pod) error {
	if pod == nil || pod.DeletionTimestamp != nil {
		// the pod has been deleted
		if err := r.handlePodDelete(ctx, podKey); err != nil {
			return errors.Wrap(err, "failed to delete endpoint for deleted pod")
		}

		return nil
	}

	if pod.Status.PodIP == "" || pod.Status.HostIP == "" {
		r.l.WithField("podKey", podKey.String()).Trace("pod missing an IP, skipping")
		return nil
	}

	podLabels, err := r.ciliumEndpointsLabels(ctx, pod)
	if err != nil {
		return errors.Wrap(err, "failed to get pod labels")
	}
	newPEP := &PodEndpoint{
		key:    podKey,
		lbls:   podLabels,
		ipv4:   pod.Status.PodIP,
		nodeIP: pod.Status.HostIP,
		// TODO: set to false if in follower mode
		processedAsLeader: true,
		uid:               pod.ObjectMeta.UID,
		podObj:            pod,
	}

	if err := r.handlePodUpsert(ctx, newPEP); err != nil {
		return errors.Wrap(err, "failed to upsert endpoint")
	}

	return nil
}

func (r *endpointReconciler) HandlePodDelete(ctx context.Context, n resource.Key) error {
	r.Mutex.Lock()
	defer r.Mutex.Unlock()
	r.l.Debug("handling pod delete with lock", zap.String("podKey", n.String()))
	return r.handlePodDelete(ctx, n)
}

func (r *endpointReconciler) handlePodDelete(ctx context.Context, n resource.Key) error {
	pep, ok := r.store.GetToDeletePod(n)
	if !ok {
		// do not do anything if we have not processed the pod
		// let endpointgc delete the CiliumEndpoint as necessary
		r.l.WithField("podKey", n.String()).Trace("pod not found in cache, skipping deletion")
		return nil
	}

	r.l.WithField("podKey", n.String()).Trace("handling pod delete")

	// delete CEP even if we haven't processed the Pod (and incremented identity reference count)
	err := r.clientset.CiliumV2().CiliumEndpoints(n.Namespace).Delete(ctx, n.Name, metav1.DeleteOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		r.l.WithError(err).WithField("podKey", n.String()).Error("failed to delete CiliumEndpoint")
		return errors.Wrap(err, "failed to delete endpoint")
	}

	r.l.WithField("podKey", n.String()).Debug("deleted CiliumEndpoint")

	// Identity reference count must be modified after CiliumEndpoint is successfully deleted.
	// Otherwise, we could decrement reference multiple times if CiliumEndpoint deletion fails and we retry this method.
	r.identityManager.DecrementReference(ctx, pep.lbls)
	r.store.DeletePod(n)

	return nil
}

func (r *endpointReconciler) handlePodUpsert(ctx context.Context, newPEP *PodEndpoint) error { //nolint:gocyclo // This function is too complex and should be refactored
	r.l.WithField("podKey", newPEP.key.String()).Trace("handling pod upsert")

	oldPEP, inCache := r.store.GetPod(newPEP.key)
	inStore := false
	if inCache {
		r.l.WithFields(logrus.Fields{
			"podKey": newPEP.key.String(),
			"pep":    oldPEP,
		}).Trace("PodEndpoint found in cache")
	} else {
		// this call will block until the store is synced with API Server
		store, err := r.ciliumEndpoints.Store(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get store")
		}

		key := resource.Key{Namespace: newPEP.key.Namespace, Name: newPEP.key.Name}
		oldCEP, ok, err := store.GetByKey(key)
		if err != nil {
			return errors.Wrap(err, "failed to get from CiliumEndpoint store")
		}

		inStore = ok

		if inStore {
			r.l.WithFields(logrus.Fields{
				"podKey":          newPEP.key.String(),
				"ownerReferences": oldCEP.ObjectMeta.OwnerReferences,
				"endpointID":      oldCEP.Status.ID,
				"identity":        oldCEP.Status.Identity,
				"networking":      oldCEP.Status.Networking,
			}).Trace("CiliumEndpoint found in store")

			if oldCEP.Status.Networking == nil || len(oldCEP.Status.Networking.Addressing) == 0 || oldCEP.Status.Networking.Addressing[0].IPV4 == "" {
				// FIXME handle IPv6 and dual-stack
				inStore = false
				r.l.WithFields(logrus.Fields{
					"podKey": newPEP.key.String(),
					"cep":    oldCEP,
				}).Warn("CiliumEndpoint has no ipv4 address, ignoring")
			} else {
				oldPEP = &PodEndpoint{
					key:        newPEP.key,
					endpointID: oldCEP.Status.ID,
					ipv4:       oldCEP.Status.Networking.Addressing[0].IPV4,
					nodeIP:     oldCEP.Status.Networking.NodeIP,
				}

				if oldCEP.Status.Identity != nil {
					oldPEP.identityID = oldCEP.Status.Identity.ID
					oldPEP.lbls = labels.NewLabelsFromModel(oldCEP.Status.Identity.Labels)
				}
			}
		}
	}

	if inCache || inStore {
		// patch existing CEP
		// specify initial IDs (might change)
		newPEP.endpointID = oldPEP.endpointID
		newPEP.identityID = oldPEP.identityID

		sameNetworking := newPEP.ipv4 == oldPEP.ipv4 && newPEP.nodeIP == oldPEP.nodeIP
		equalLabels := newPEP.lbls.Equals(oldPEP.lbls)

		r.l.WithFields(logrus.Fields{
			"podKey":         newPEP.key.String(),
			"inCache":        inCache,
			"sameNetworking": sameNetworking,
			"equalLabels":    equalLabels,
			"oldLbls":        oldPEP.lbls,
			"newLbls":        newPEP.lbls,
		}).Trace("patching CiliumEndpoint")

		// allocate new identity if labels have changed or if we haven't assigned an identity ID to this pod yet
		// TODO when implementing follower check if inCache or processedAsLeader
		shouldAllocateNewIdentity := !inCache || !equalLabels || newPEP.identityID == 0
		if shouldAllocateNewIdentity {
			r.l.WithFields(logrus.Fields{
				"podKey": newPEP.key.String(),
				"pep":    oldPEP,
			}).Trace("creating new identity for pod")

			identityID, err := r.identityManager.GetIdentityAndIncrementReference(ctx, newPEP.lbls)
			if err != nil {
				return errors.Wrap(err, "failed to get identity ID for updated/uncached pod")
			}

			newPEP.identityID = identityID
		}

		if sameNetworking && equalLabels {
			// nothing to do. pod already has a CEP with the same networking and labels
			r.l.WithField("podKey", newPEP.key.String()).Trace("pod already processed")
			r.store.AddPod(newPEP)
			return nil
		}

		if !sameNetworking {
			r.l.WithField("podKey", newPEP.key.String()).Trace("pod networking has changed")
			// change endpoint id since networking has changed
			// FIXME use endpoint allocator to get new endpoint ID
			newPEP.endpointID++
		}

		status := newPEP.endpointStatus()
		replaceCEPStatus := []k8s.JSONPatch{
			{
				OP:    "replace",
				Path:  "/status",
				Value: status,
			},
		}

		if !sameNetworking && useOwnerReferences {
			// Pod has changed, update ownerReferences.
			// This might be a pointless patch call (resulting in CEP not found)
			// since the CEP will be deleted when the original OwnerReference Pod is deleted from API Server.
			replaceCEPStatus = append(replaceCEPStatus, k8s.JSONPatch{
				OP:   "test",
				Path: "/metadata/ownerReferences",
				Value: []metav1.OwnerReference{
					{
						APIVersion: "v1",
						Kind:       "Pod",
						Name:       newPEP.key.Name,
						UID:        newPEP.uid,
					},
				},
			})
		}

		createStatusPatch, err := json.Marshal(replaceCEPStatus)
		if err != nil {
			r.l.WithFields(logrus.Fields{
				"podKey": newPEP.key.String(),
				"pep":    newPEP,
				"uid":    newPEP.uid,
			}).Debug("marshalling status failed")

			if shouldAllocateNewIdentity {
				// decrement reference for new identity
				r.l.WithField("podKey", newPEP.key.String()).Trace("marshal failed, decrementing reference for new identity")
				r.identityManager.DecrementReference(ctx, newPEP.lbls)
			}

			// TODO release newly allocated endpoint ID if networking not the same

			return errors.Wrap(err, "failed to marshal status patch")
		}

		_, err = r.clientset.CiliumV2().CiliumEndpoints(newPEP.key.Namespace).Patch(ctx, newPEP.key.Name, types.JSONPatchType, createStatusPatch, metav1.PatchOptions{})
		if (err == nil || k8serrors.IsNotFound(err)) && inCache && !equalLabels {
			// decrement reference for old identity
			// TODO when implementing follower check if processedAsLeader
			r.identityManager.DecrementReference(ctx, oldPEP.lbls)
		}

		if err == nil {
			// Update the pod in cache and return
			r.store.AddPod(newPEP)
			return nil
		}
		if shouldAllocateNewIdentity {
			// Decrement reference for new identity.
			// May end up incrementing reference count for this same identity again if we try to create the CEP below.
			// No downside to decrementing reference here and then incrementing again below (will not affect API Server).
			r.l.WithField("podKey", newPEP.key.String()).Trace("patch unsuccessful, decrementing reference for new identity")
			r.identityManager.DecrementReference(ctx, newPEP.lbls)
		}

		// TODO if networking changed, release newly allocated endpoint ID.
		// May end up getting another endpoint ID below if we try to create the CEP below.
		// No downside to this.

		if !k8serrors.IsNotFound(err) {
			r.l.WithError(err).WithFields(logrus.Fields{
				"podKey": newPEP.key.String(),
				"pep":    newPEP,
				"uid":    newPEP.uid,
			}).Error("failed to patch CiliumEndpoint")

			return errors.Wrap(err, "failed to patch endpoint")
		}

		r.l.WithField("podKey", newPEP.key.String()).Debug("patch unsuccessful because CiliumEndpoint is not in API Server. now creating CiliumEndpoint")

		// Endpoint was not found, create it below.
		// Make sure the pod does not exist in the cache so that we don't try to patch it again (in case of a retry after a failure below).
		// The CEP should (eventually) not exist in the CEP store too since API Server says it does not exist.
		r.store.DeletePod(newPEP.key)
	}

	// create CEP
	// get new identity ID
	identityID, err := r.identityManager.GetIdentityAndIncrementReference(ctx, newPEP.lbls)
	if err != nil {
		return errors.Wrap(err, "failed to get identity ID for new pod")
	}

	newPEP.identityID = identityID
	// FIXME specify endpoint ID with allocator
	newPEP.endpointID = 1

	// create CiliumEndpoint
	newCEP := &ciliumv2.CiliumEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:      newPEP.key.Name,
			Namespace: newPEP.key.Namespace,
		},
		Status: newPEP.endpointStatus(),
	}

	if useOwnerReferences {
		newCEP.ObjectMeta.OwnerReferences = []metav1.OwnerReference{
			{
				APIVersion: "v1",
				Kind:       "Pod",
				Name:       newPEP.key.Name,
				UID:        newPEP.uid,
			},
		}
	}

	_, err = r.clientset.CiliumV2().CiliumEndpoints(newPEP.key.Namespace).Create(ctx, newCEP, metav1.CreateOptions{})
	if err != nil {
		r.l.WithError(err).WithField("podKey", newPEP.key.String()).Error("failed to create CiliumEndpoint")
		r.identityManager.DecrementReference(ctx, newPEP.lbls)
		// FIXME release newly allocated endpoint ID
		return errors.Wrap(err, "failed to create endpoint")
	}

	r.l.WithField("podKey", newPEP.key.String()).Debug("created CiliumEndpoint")
	r.store.AddPod(newPEP)
	return nil
}

func (r *endpointReconciler) reconcileNamespace(ctx context.Context, namespace *slim_corev1.Namespace) error {
	if namespace == nil {
		return nil
	}

	// check if namespace is being deleted
	if namespace.DeletionTimestamp != nil {
		return r.handleNamespaceDelete(ctx, namespace.GetName())
	}

	// check if namespace is in cache
	oldNs, ok := r.store.GetNamespace(namespace.GetName())
	if !ok {
		r.l.Debug("Adding new namespace to cache", zap.String("namespace ", namespace.GetName()))
		// if this is the first time we see this namespace, add it to cache
		// there might not be any pods in this namespace yet
		r.store.AddNamespace(namespace)
		return nil
	}

	if oldNs.GetResourceVersion() == namespace.GetResourceVersion() {
		r.l.Debug("Namespace already processed", zap.String("namespace ", namespace.GetName()))
		return nil
	}

	if reflect.DeepEqual(oldNs.Labels, namespace.Labels) {
		r.l.Debug("Namespace labels are the same", zap.String("namespace ", namespace.GetName()))
		return nil
	}

	r.l.Debug("Updating namespace in cache", zap.String("namespace ", namespace.GetName()))
	r.store.AddNamespace(namespace)

	// now get all pods and update them as well
	err := r.ReconcilePodsInNamespace(ctx, namespace.GetName())
	if err != nil {
		return errors.Wrap(err, "failed to reconcile pods in namespace"+namespace.GetName())
	}
	return nil
}

func (r *endpointReconciler) handleNamespaceDelete(_ context.Context, namespaceName string) error {
	_, ok := r.store.GetNamespace(namespaceName)
	if !ok {
		r.l.Debug("Adding new namespace to cache", zap.String("namespace ", namespaceName))
		return nil
	}

	r.l.Debug("Deleting namespace from cache", zap.String("namespace ", namespaceName))
	// Ignore deleting the pods for this NS, pod controller will eventually clean it up.
	// Once deleting all the pods in the namespace, delete the namespace
	r.store.DeleteNamespace(namespaceName)
	return nil
}

func (r *endpointReconciler) ciliumEndpointsLabels(ctx context.Context, pod *slim_corev1.Pod) (labels.Labels, error) {
	// Get namespace labels from cache
	ns, ok := r.store.GetNamespace(pod.Namespace)
	var err error
	if !ok {
		ns, err = r.ciliumSlimClientSet.CoreV1().Namespaces().Get(ctx, pod.Namespace, metav1.GetOptions{})
		if err != nil {
			r.l.WithError(err).WithFields(logrus.Fields{
				"podKey": pod.Name,
				"ns":     pod.Namespace,
			}).Error("failed to get namespace")
			return nil, errors.Wrap(err, "failed to get namespace")
		}
		r.store.AddNamespace(ns)
	}
	_, ciliumLabels, _, err := k8s.GetPodMetadata(ns, pod)
	if err != nil {
		r.l.WithError(err).WithFields(logrus.Fields{
			"podKey": pod.Name,
			"ns":     pod.Namespace,
		}).Error("failed to get pod metadata")
		return nil, errors.Wrap(err, "failed to get pod metadata")
	}
	lbls := make(labels.Labels, len(ciliumLabels))
	for k, v := range ciliumLabels {
		lbls[k] = labels.Label{
			Key:    k,
			Value:  v,
			Source: labels.LabelSourceK8s,
		}
	}
	return lbls, nil
}
