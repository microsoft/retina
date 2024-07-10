// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package endpointcontroller

import (
	"sync"

	ciliumv2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	"github.com/cilium/cilium/pkg/k8s/resource"
	slim_corev1 "github.com/cilium/cilium/pkg/k8s/slim/k8s/api/core/v1"
	"github.com/cilium/cilium/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
)

// PodEndpoint represents a Pod/CiliumEndpoint
type PodEndpoint struct {
	key        resource.Key
	endpointID int64
	identityID int64
	lbls       labels.Labels
	ipv4       string
	nodeIP     string
	// processedAsLeader will be used when implementing follower mode.
	// Follower would process all pod events, but it won't use IdentityManager (increment identity references) until leading
	// since IdentityManager implicitly affects API Server.
	processedAsLeader bool

	uid types.UID

	// toDelete is used to mark the pod for deletion
	toDelete bool

	// podObj is the pod object
	podObj *slim_corev1.Pod
}

func (pep *PodEndpoint) endpointStatus() ciliumv2.EndpointStatus {
	return ciliumv2.EndpointStatus{
		ID: pep.endpointID,
		Identity: &ciliumv2.EndpointIdentity{
			ID:     pep.identityID,
			Labels: pep.lbls.GetPrintableModel(),
		},
		Networking: &ciliumv2.EndpointNetworking{
			NodeIP: pep.nodeIP,
			Addressing: ciliumv2.AddressPairList{
				{
					IPV4: pep.ipv4,
				},
			},
		},

		State: "ready",
	}
}

func (pep *PodEndpoint) deepCopy() *PodEndpoint {
	return &PodEndpoint{
		key:               pep.key,
		endpointID:        pep.endpointID,
		identityID:        pep.identityID,
		lbls:              pep.lbls,
		ipv4:              pep.ipv4,
		nodeIP:            pep.nodeIP,
		processedAsLeader: pep.processedAsLeader,
		uid:               pep.uid,
		podObj:            pep.podObj,
	}
}

type Store struct { //nolint:gocritic // This should be rewritten to limit exposure of mutex to external packages.
	*sync.RWMutex

	// Pods is a map of Pod key to PodEndpoint
	// this is the expected endpoint state for the pod
	// and is used to determine if the pod needs to be updated
	Pods map[resource.Key]*PodEndpoint

	// Namespaces is a map of Namespace name to Namespace
	// this is used to determine if the namespace needs to be updated
	Namespaces map[string]*slim_corev1.Namespace
}

func NewStore() *Store {
	return &Store{
		RWMutex:    &sync.RWMutex{},
		Pods:       make(map[resource.Key]*PodEndpoint),
		Namespaces: make(map[string]*slim_corev1.Namespace),
	}
}

func (s *Store) AddPod(pod *PodEndpoint) {
	s.Lock()
	defer s.Unlock()
	s.Pods[pod.key] = pod
}

func (s *Store) AddNamespace(namespace *slim_corev1.Namespace) {
	s.Lock()
	defer s.Unlock()
	s.Namespaces[namespace.GetName()] = namespace
}

func (s *Store) GetPod(key resource.Key) (*PodEndpoint, bool) {
	s.RLock()
	defer s.RUnlock()
	pod, ok := s.Pods[key]
	return pod, ok
}

func (s *Store) GetToDeletePod(key resource.Key) (*PodEndpoint, bool) {
	s.Lock()
	defer s.Unlock()
	pod, ok := s.Pods[key]
	if ok {
		pod.toDelete = true
	}
	return pod, ok
}

func (s *Store) GetNamespace(key string) (*slim_corev1.Namespace, bool) {
	s.RLock()
	defer s.RUnlock()
	namespace, ok := s.Namespaces[key]
	return namespace, ok
}

func (s *Store) DeletePod(key resource.Key) {
	s.Lock()
	defer s.Unlock()
	delete(s.Pods, key)
}

func (s *Store) DeleteNamespace(key string) {
	s.Lock()
	defer s.Unlock()
	delete(s.Namespaces, key)
}

func (s *Store) ListPodKeysByNamespace(namespace string) []resource.Key {
	s.RLock()
	defer s.RUnlock()
	keys := make([]resource.Key, 0)
	for key, pod := range s.Pods {
		if pod.key.Namespace == namespace {
			keys = append(keys, key)
		}
	}
	return keys
}
