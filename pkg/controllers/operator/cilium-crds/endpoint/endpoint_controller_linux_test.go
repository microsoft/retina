package endpointcontroller

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	ciliumv2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	ciliumclient "github.com/cilium/cilium/pkg/k8s/client/testutils"
	"github.com/cilium/cilium/pkg/k8s/resource"
	v1 "github.com/cilium/cilium/pkg/k8s/slim/k8s/api/core/v1"
	slim_metav1 "github.com/cilium/cilium/pkg/k8s/slim/k8s/apis/meta/v1"
	corev1 "github.com/cilium/cilium/pkg/k8s/slim/k8s/client/clientset/versioned/typed/core/v1"
	"github.com/cilium/cilium/pkg/labels"
	"github.com/cilium/cilium/pkg/option"
	"github.com/microsoft/retina/pkg/controllers/operator/cilium-crds/cache"
	ciliumutil "github.com/microsoft/retina/pkg/utils/testutil/cilium"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func podTestX() (resource.Key, *v1.Pod) {
	return resource.Key{
			Name:      "x",
			Namespace: "test",
		},
		&v1.Pod{
			ObjectMeta: slim_metav1.ObjectMeta{
				UID: "111",
				Labels: map[string]string{
					"k1": "v1",
				},
				Name:      "x",
				Namespace: "test",
			},
			Status: v1.PodStatus{
				PodIP:  "1.2.3.4",
				HostIP: "10.0.0.1",
			},
		}
}

func createNamespace(c corev1.CoreV1Interface) {
	// Create the namespace.
	err, _ := c.Namespaces().Create(context.TODO(), &v1.Namespace{
		ObjectMeta: slim_metav1.ObjectMeta{
			Name: "test",
		},
	}, metav1.CreateOptions{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: "v1",
		},
	})
	if err != nil {
		fmt.Printf("Error creating namespace %s:\n", err)
	}
}

func TestPodCreate(t *testing.T) {
	r, ciliumEndpoints := newTestEndpointReconciler(t)

	createNamespace(r.ciliumSlimClientSet.CoreV1())
	podKey, pod := podTestX()

	require.NoError(t, r.ReconcilePod(context.TODO(), podKey, pod))
	key := resource.Key{Name: "x", Namespace: "test"}
	pep, ok := r.store.GetPod(key)
	require.True(t, ok)
	require.NotNil(t, pep)
	identityID := pep.identityID
	require.Greater(t, identityID, int64(0))

	var expectedEndpointID int64 = 1 // FIXME switch to mock allocator once endpoint IDs are allocated by the operator
	expectedCache := map[resource.Key]*PodEndpoint{
		key: {
			key:        key,
			endpointID: expectedEndpointID,
			identityID: identityID,
			lbls: labels.Labels{
				"k1": labels.Label{
					Key:    "k1",
					Value:  "v1",
					Source: "k8s",
				},
				"io.kubernetes.pod.namespace": labels.Label{
					Key:    "io.kubernetes.pod.namespace",
					Value:  "test",
					Source: "k8s",
				},
				"io.cilium.k8s.policy.cluster": labels.Label{
					Key:    "io.cilium.k8s.policy.cluster",
					Value:  "",
					Source: "k8s",
				},
			},
			ipv4:              "1.2.3.4",
			nodeIP:            "10.0.0.1",
			processedAsLeader: true,
			uid:               "111",
			podObj:            pod,
		},
	}

	require.Equal(t, expectedCache, r.store.Pods)
	cep := getAndAssertCEPExists(t, ciliumEndpoints, pod)
	expectedCEP := &ciliumv2.CiliumEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "x",
			Namespace: "test",
		},
		Status: ciliumv2.EndpointStatus{
			ID: expectedEndpointID,
			Identity: &ciliumv2.EndpointIdentity{
				ID:     identityID,
				Labels: []string{"k8s:io.cilium.k8s.policy.cluster", "k8s:io.kubernetes.pod.namespace=test", "k8s:k1=v1"},
			},
			Networking: &ciliumv2.EndpointNetworking{
				NodeIP: "10.0.0.1",
				Addressing: ciliumv2.AddressPairList{
					{
						IPV4: "1.2.3.4",
					},
				},
			},

			State: "ready",
		},
	}
	t.Logf("%+v", cep.Status.Identity.Labels)
	require.Equal(t, expectedCEP, cep)
}

func TestPodDelete(t *testing.T) {
	r, ciliumEndpoints := newTestEndpointReconciler(t)

	createNamespace(r.ciliumSlimClientSet.CoreV1())
	podKey, pod := podTestX()

	require.NoError(t, r.ReconcilePod(context.TODO(), podKey, pod))
	pod = nil
	require.NoError(t, r.ReconcilePod(context.TODO(), podKey, pod))
	assertCEPDoesNotExist(t, ciliumEndpoints, podKey)
}

func TestPodDeleteNoOp(t *testing.T) {
	r, ciliumEndpoints := newTestEndpointReconciler(t)

	createNamespace(r.ciliumSlimClientSet.CoreV1())
	podKey, _ := podTestX()

	require.NoError(t, r.ReconcilePod(context.TODO(), podKey, nil))
	assertCEPDoesNotExist(t, ciliumEndpoints, podKey)
}

func TestPodLabelsChanged(t *testing.T) {
	r, ciliumEndpoints := newTestEndpointReconciler(t)

	createNamespace(r.ciliumSlimClientSet.CoreV1())
	podKey, pod := podTestX()

	require.NoError(t, r.ReconcilePod(context.TODO(), podKey, pod))
	key := resource.Key{Name: "x", Namespace: "test"}
	pep, ok := r.store.GetPod(key)
	require.True(t, ok)
	require.NotNil(t, pep)
	identityID := pep.identityID
	require.Greater(t, identityID, int64(0))

	var expectedEndpointID int64 = 1 // FIXME switch to mock allocator once endpoint IDs are allocated by the operator
	expectedCache := map[resource.Key]*PodEndpoint{
		key: {
			key:        key,
			endpointID: expectedEndpointID,
			identityID: identityID,
			lbls: labels.Labels{
				"k1": labels.Label{
					Key:    "k1",
					Value:  "v1",
					Source: "k8s",
				},
				"io.kubernetes.pod.namespace": labels.Label{
					Key:    "io.kubernetes.pod.namespace",
					Value:  "test",
					Source: "k8s",
				},
				"io.cilium.k8s.policy.cluster": labels.Label{
					Key:    "io.cilium.k8s.policy.cluster",
					Value:  "",
					Source: "k8s",
				},
			},
			ipv4:              "1.2.3.4",
			nodeIP:            "10.0.0.1",
			processedAsLeader: true,
			uid:               "111",
			podObj:            pod,
		},
	}

	require.Equal(t, expectedCache, r.store.Pods)
	cep := getAndAssertCEPExists(t, ciliumEndpoints, pod)
	expectedCEP := &ciliumv2.CiliumEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "x",
			Namespace: "test",
		},
		Status: ciliumv2.EndpointStatus{
			ID: expectedEndpointID,
			Identity: &ciliumv2.EndpointIdentity{
				ID:     identityID,
				Labels: []string{"k8s:io.cilium.k8s.policy.cluster", "k8s:io.kubernetes.pod.namespace=test", "k8s:k1=v1"},
			},
			Networking: &ciliumv2.EndpointNetworking{
				NodeIP: "10.0.0.1",
				Addressing: ciliumv2.AddressPairList{
					{
						IPV4: "1.2.3.4",
					},
				},
			},

			State: "ready",
		},
	}
	t.Logf("%+v", cep.Status.Identity.Labels)
	require.Equal(t, expectedCEP, cep)

	podKeyNew, podNew := podTestX()
	podNew.ObjectMeta.Labels["k1"] = "v2"
	podNew.ObjectMeta.Labels["k2"] = "v2"

	require.NoError(t, r.ReconcilePod(context.TODO(), podKeyNew, podNew))

	pep, ok = r.store.GetPod(key)
	require.True(t, ok)
	require.NotNil(t, pep)
	require.NotEqual(t, identityID, pep.identityID)
	identityID = pep.identityID
	expectedCacheNew := map[resource.Key]*PodEndpoint{
		key: {
			key:        key,
			endpointID: expectedEndpointID,
			identityID: identityID,
			lbls: labels.Labels{
				"k2": labels.Label{
					Key:    "k2",
					Value:  "v2",
					Source: "k8s",
				},
				"k1": labels.Label{
					Key:    "k1",
					Value:  "v2",
					Source: "k8s",
				},
				"io.kubernetes.pod.namespace": labels.Label{
					Key:    "io.kubernetes.pod.namespace",
					Value:  "test",
					Source: "k8s",
				},
				"io.cilium.k8s.policy.cluster": labels.Label{
					Key:    "io.cilium.k8s.policy.cluster",
					Value:  "",
					Source: "k8s",
				},
			},
			ipv4:              "1.2.3.4",
			nodeIP:            "10.0.0.1",
			processedAsLeader: true,
			uid:               "111",
			podObj:            podNew,
		},
	}

	require.Equal(t, expectedCacheNew, r.store.Pods)
	cep = getAndAssertCEPExists(t, ciliumEndpoints, podNew)
	expectedCEP = &ciliumv2.CiliumEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "x",
			Namespace: "test",
		},
		Status: ciliumv2.EndpointStatus{
			ID: expectedEndpointID,
			Identity: &ciliumv2.EndpointIdentity{
				ID:     identityID,
				Labels: []string{"k8s:io.cilium.k8s.policy.cluster", "k8s:io.kubernetes.pod.namespace=test", "k8s:k1=v2", "k8s:k2=v2"},
			},
			Networking: &ciliumv2.EndpointNetworking{
				NodeIP: "10.0.0.1",
				Addressing: ciliumv2.AddressPairList{
					{
						IPV4: "1.2.3.4",
					},
				},
			},

			State: "ready",
		},
	}
	t.Logf("%+v", cep.Status.Identity.Labels)
	require.Equal(t, expectedCEP, cep)
}

func TestPodNetworkingChanged(t *testing.T) {
	r, ciliumEndpoints := newTestEndpointReconciler(t)

	createNamespace(r.ciliumSlimClientSet.CoreV1())
	podKey, pod := podTestX()
	var expectedEndpointID int64 = 1
	require.NoError(t, r.ReconcilePod(context.TODO(), podKey, pod))
	cep := getAndAssertCEPExists(t, ciliumEndpoints, pod)
	key := resource.Key{Name: "x", Namespace: "test"}
	pep, ok := r.store.GetPod(key)
	require.True(t, ok)
	require.NotNil(t, pep)
	identityID := pep.identityID
	expectedCEP := &ciliumv2.CiliumEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "x",
			Namespace: "test",
		},
		Status: ciliumv2.EndpointStatus{
			ID: expectedEndpointID,
			Identity: &ciliumv2.EndpointIdentity{
				ID:     identityID,
				Labels: []string{"k8s:io.cilium.k8s.policy.cluster", "k8s:io.kubernetes.pod.namespace=test", "k8s:k1=v1"},
			},
			Networking: &ciliumv2.EndpointNetworking{
				NodeIP: "10.0.0.1",
				Addressing: ciliumv2.AddressPairList{
					{
						IPV4: "1.2.3.4",
					},
				},
			},

			State: "ready",
		},
	}
	t.Logf("%+v", cep.Status.Identity.Labels)
	require.Equal(t, expectedCEP, cep)

	podKeyNew, podNew := podTestX()
	podNew.Status.PodIP = "4.3.2.1"

	require.NoError(t, r.ReconcilePod(context.TODO(), podKeyNew, podNew))
	expectedEndpointID++

	pep, ok = r.store.GetPod(key)
	require.True(t, ok)
	require.NotNil(t, pep)
	identityID = pep.identityID
	expectedCacheNew := map[resource.Key]*PodEndpoint{
		key: {
			key:        key,
			endpointID: expectedEndpointID,
			identityID: identityID,
			lbls: labels.Labels{
				"k1": labels.Label{
					Key:    "k1",
					Value:  "v1",
					Source: "k8s",
				},
				"io.kubernetes.pod.namespace": labels.Label{
					Key:    "io.kubernetes.pod.namespace",
					Value:  "test",
					Source: "k8s",
				},
				"io.cilium.k8s.policy.cluster": labels.Label{
					Key:    "io.cilium.k8s.policy.cluster",
					Value:  "",
					Source: "k8s",
				},
			},
			ipv4:              "4.3.2.1",
			nodeIP:            "10.0.0.1",
			processedAsLeader: true,
			uid:               "111",
			podObj:            podNew,
		},
	}
	require.Equal(t, expectedCacheNew, r.store.Pods)
	cep = getAndAssertCEPExists(t, ciliumEndpoints, podNew)

	expectedCEPNew := &ciliumv2.CiliumEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "x",
			Namespace: "test",
		},
		Status: ciliumv2.EndpointStatus{
			ID: expectedEndpointID,
			Identity: &ciliumv2.EndpointIdentity{
				ID:     identityID,
				Labels: []string{"k8s:io.cilium.k8s.policy.cluster", "k8s:io.kubernetes.pod.namespace=test", "k8s:k1=v1"},
			},
			Networking: &ciliumv2.EndpointNetworking{
				NodeIP: "10.0.0.1",
				Addressing: ciliumv2.AddressPairList{
					{
						IPV4: "4.3.2.1",
					},
				},
			},

			State: "ready",
		},
	}
	require.Equal(t, expectedCEPNew, cep)

	// Changing the Node IP
	podNew.Status.HostIP = "10.10.10.10"

	require.NoError(t, r.ReconcilePod(context.TODO(), podKeyNew, podNew))
	expectedEndpointID++

	pep, ok = r.store.GetPod(key)
	require.True(t, ok)
	require.NotNil(t, pep)
	identityID = pep.identityID
	expectedCacheNew = map[resource.Key]*PodEndpoint{
		key: {
			key:        key,
			endpointID: expectedEndpointID,
			identityID: identityID,
			lbls: labels.Labels{
				"k1": labels.Label{
					Key:    "k1",
					Value:  "v1",
					Source: "k8s",
				},
				"io.kubernetes.pod.namespace": labels.Label{
					Key:    "io.kubernetes.pod.namespace",
					Value:  "test",
					Source: "k8s",
				},
				"io.cilium.k8s.policy.cluster": labels.Label{
					Key:    "io.cilium.k8s.policy.cluster",
					Value:  "",
					Source: "k8s",
				},
			},
			ipv4:              "4.3.2.1",
			nodeIP:            "10.10.10.10",
			processedAsLeader: true,
			uid:               "111",
			podObj:            podNew,
		},
	}
	require.Equal(t, expectedCacheNew, r.store.Pods)
	cep = getAndAssertCEPExists(t, ciliumEndpoints, podNew)

	expectedCEPNew = &ciliumv2.CiliumEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "x",
			Namespace: "test",
		},
		Status: ciliumv2.EndpointStatus{
			ID: expectedEndpointID,
			Identity: &ciliumv2.EndpointIdentity{
				ID:     identityID,
				Labels: []string{"k8s:io.cilium.k8s.policy.cluster", "k8s:io.kubernetes.pod.namespace=test", "k8s:k1=v1"},
			},
			Networking: &ciliumv2.EndpointNetworking{
				NodeIP: "10.10.10.10",
				Addressing: ciliumv2.AddressPairList{
					{
						IPV4: "4.3.2.1",
					},
				},
			},

			State: "ready",
		},
	}
	require.Equal(t, expectedCEPNew, cep)
}

func TestNamespaceDelete(t *testing.T) {
	r, ciliumEndpoints := newTestEndpointReconciler(t)

	createNamespace(r.ciliumSlimClientSet.CoreV1())
	podKey, pod := podTestX()

	require.NoError(t, r.ReconcilePod(context.TODO(), podKey, pod))
	require.NoError(t, r.reconcileNamespace(context.TODO(), &v1.Namespace{
		ObjectMeta: slim_metav1.ObjectMeta{
			Name:              "test",
			DeletionTimestamp: &slim_metav1.Time{Time: time.Now()},
		},
	}))

	require.Empty(t, r.store.Namespaces)
	// deleting namespace does not delete the endpoint in cache.
	// we will let pod controller delete the endpoint
	_ = getAndAssertCEPExists(t, ciliumEndpoints, pod)
}

func TestNamespaceUpdate(t *testing.T) {
	r, ciliumEndpoints := newTestEndpointReconciler(t)

	createNamespace(r.ciliumSlimClientSet.CoreV1())
	podKey, pod := podTestX()

	require.NoError(t, r.ReconcilePod(context.TODO(), podKey, pod))
	require.NoError(t, r.reconcileNamespace(context.TODO(), &v1.Namespace{
		ObjectMeta: slim_metav1.ObjectMeta{
			Name:            "test",
			Labels:          map[string]string{"k1": "v1"},
			ResourceVersion: "3",
		},
	}))

	cep := getAndAssertCEPExists(t, ciliumEndpoints, pod)
	var expectedEndpointID int64 = 1
	require.NoError(t, r.ReconcilePod(context.TODO(), podKey, pod))
	key := resource.Key{Name: "x", Namespace: "test"}
	pep, ok := r.store.GetPod(key)
	require.True(t, ok)
	require.NotNil(t, pep)
	identityID := pep.identityID
	expectedCEP := &ciliumv2.CiliumEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "x",
			Namespace: "test",
		},
		Status: ciliumv2.EndpointStatus{
			ID: expectedEndpointID,
			Identity: &ciliumv2.EndpointIdentity{
				ID:     identityID,
				Labels: []string{"k8s:io.cilium.k8s.namespace.labels.k1=v1", "k8s:io.cilium.k8s.policy.cluster", "k8s:io.kubernetes.pod.namespace=test", "k8s:k1=v1"},
			},
			Networking: &ciliumv2.EndpointNetworking{
				NodeIP: "10.0.0.1",
				Addressing: ciliumv2.AddressPairList{
					{
						IPV4: "1.2.3.4",
					},
				},
			},

			State: "ready",
		},
	}
	t.Logf("%+v", cep.Status.Identity.Labels)
	require.Equal(t, expectedCEP, cep)
}

func TestUpdateFailurePodLabelsChanged(_ *testing.T) {
}

func TestUpdateFailurePodNetworkingChanged(_ *testing.T) {
}

func TestBootupNoOp(_ *testing.T) {
}

func TestBootupPodLabelsChanged(_ *testing.T) {
}

func TestBootupPodNetworkingChanged(_ *testing.T) {
}

func TestBootupUpdateFailurePodLabelsChanged(_ *testing.T) {
}

func TestBootupUpdateFailurePodNetworkingChanged(_ *testing.T) {
}

func TestPodWithoutIP(t *testing.T) {
	r, ciliumEndpoints := newTestEndpointReconciler(t)

	createNamespace(r.ciliumSlimClientSet.CoreV1())

	podKey, pod := podTestX()
	pod.Status.PodIP = ""
	require.NoError(t, r.ReconcilePod(context.TODO(), podKey, pod))
	assertCEPDoesNotExist(t, ciliumEndpoints, podKey)

	podKey, pod = podTestX()
	pod.Status.HostIP = ""
	require.NoError(t, r.ReconcilePod(context.TODO(), podKey, pod))
	assertCEPDoesNotExist(t, ciliumEndpoints, podKey)
}

func TestStoreFailure(t *testing.T) {
	r, ciliumEndpoints := newTestEndpointReconciler(t)

	ciliumEndpoints.FailOnNextStoreCall()

	createNamespace(r.ciliumSlimClientSet.CoreV1())
	podKey, pod := podTestX()
	require.Error(t, r.ReconcilePod(context.TODO(), podKey, pod))
}

func TestSortedLabels(t *testing.T) {
	r, _ := newTestEndpointReconciler(t)

	createNamespace(r.ciliumSlimClientSet.CoreV1())

	pod := cache.PodCacheObject{
		Key: resource.Key{
			Namespace: "test",
			Name:      "a",
		},
		Pod: &v1.Pod{
			ObjectMeta: slim_metav1.ObjectMeta{
				Labels: map[string]string{
					"k3": "v3",
					"k1": "v1",
					"k2": "v2",
					"k5": "v5",
					"k4": "v4",
				},
				Namespace: "test",
				Name:      "a",
			},
		},
	}

	lbls, err := r.ciliumEndpointsLabels(context.Background(), pod.Pod)
	require.NoError(t, err)

	expected := make(labels.Labels)
	expected["k1"] = labels.Label{
		Key:    "k1",
		Value:  "v1",
		Source: "k8s",
	}
	expected["k2"] = labels.Label{
		Key:    "k2",
		Value:  "v2",
		Source: "k8s",
	}
	expected["k3"] = labels.Label{
		Key:    "k3",
		Value:  "v3",
		Source: "k8s",
	}
	expected["k4"] = labels.Label{
		Key:    "k4",
		Value:  "v4",
		Source: "k8s",
	}
	expected["k5"] = labels.Label{
		Key:    "k5",
		Value:  "v5",
		Source: "k8s",
	}
	expected["io.kubernetes.pod.namespace"] = labels.Label{
		Key:    "io.kubernetes.pod.namespace",
		Value:  "test",
		Source: "k8s",
	}
	expected["io.cilium.k8s.policy.cluster"] = labels.Label{
		Key:    "io.cilium.k8s.policy.cluster",
		Value:  "",
		Source: "k8s",
	}

	require.Equal(t, expected, lbls)
	require.Equal(t, "k8s:io.cilium.k8s.policy.cluster,k8s:io.kubernetes.pod.namespace=test,k8s:k1=v1,k8s:k2=v2,k8s:k3=v3,k8s:k4=v4,k8s:k5=v5", lbls.String())
}

func newTestEndpointReconciler(t *testing.T) (*endpointReconciler, *ciliumutil.MockResource[*ciliumv2.CiliumEndpoint]) {
	t.Helper()
	l := slog.Default()
	ciliumEndpoints := ciliumutil.NewMockResource[*ciliumv2.CiliumEndpoint](l)

	fakeClientSet, _ := ciliumclient.NewFakeClientset(slog.Default())

	m := ciliumutil.NewMockVersionedClient(l, ciliumEndpoints)
	r := &endpointReconciler{
		l:                   l,
		clientset:           m,
		podEvents:           nil,
		ciliumEndpoints:     ciliumEndpoints,
		ciliumSlimClientSet: fakeClientSet.SlimFakeClientset,
		k8sClient:           fakeClientSet,
		store:               NewStore(),
		Mutex:               &sync.Mutex{},
		windowsNodes:        make(map[string]bool),
	}

	// make sure to use CRD mode (this is referenced in InitIdentityAllocator)
	option.Config.IdentityAllocationMode = option.IdentityAllocationModeCRD
	// Use the Cilium fake clientset for identity manager since it has proper watch support
	im, err := NewIdentityManager(context.Background(), l, fakeClientSet.CiliumFakeClientset, false)
	require.NoError(t, err)
	r.identityManager = im

	return r, ciliumEndpoints
}

func assertCEPDoesNotExist(t *testing.T, ciliumEndpoints *ciliumutil.MockResource[*ciliumv2.CiliumEndpoint], key resource.Key) {
	t.Helper()
	_, ok, err := ciliumEndpoints.GetByKey(key)
	require.NoError(t, err, "error getting from store")
	require.False(t, ok, "cilium endpoint should not exist")
}

func getAndAssertCEPExists(t *testing.T, ciliumEndpoints *ciliumutil.MockResource[*ciliumv2.CiliumEndpoint], pod *v1.Pod) *ciliumv2.CiliumEndpoint {
	t.Helper()
	key := resource.Key{Name: pod.Name, Namespace: pod.Namespace}
	cep, ok, err := ciliumEndpoints.GetByKey(key)
	require.NoError(t, err)
	require.True(t, ok, "cilium endpoint should exist")
	return cep
}

func createNode(t *testing.T, c corev1.CoreV1Interface, name, os string) {
	t.Helper()
	_, err := c.Nodes().Create(context.TODO(), &v1.Node{
		ObjectMeta: slim_metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{"kubernetes.io/os": os},
		},
	}, metav1.CreateOptions{})
	require.NoError(t, err)
}

func podOnNode(name, node, ip string) (resource.Key, *v1.Pod) {
	key, pod := podTestX()
	key.Name = name
	pod.Name = name
	pod.UID = "uid-" + types.UID(name)
	pod.Spec.NodeName = node
	pod.Status.PodIP = ip
	return key, pod
}

func TestWindowsOnlyEndpointsSkipsLinuxPods(t *testing.T) {
	r, ciliumEndpoints := newTestEndpointReconciler(t)
	r.cfg = Config{WindowsOnlyEndpoints: true, WindowsCIDRLabels: true}
	createNamespace(r.ciliumSlimClientSet.CoreV1())
	createNode(t, r.ciliumSlimClientSet.CoreV1(), "linux-node", "linux")
	createNode(t, r.ciliumSlimClientSet.CoreV1(), "win-node", "windows")

	linuxKey, linuxPod := podOnNode("linux-pod", "linux-node", "10.244.0.5")
	require.NoError(t, r.ReconcilePod(context.TODO(), linuxKey, linuxPod))
	_, ok := r.store.GetPod(linuxKey)
	require.False(t, ok, "linux pod must not be managed")
	assertCEPDoesNotExist(t, ciliumEndpoints, linuxKey)
	require.NoError(t, r.HandlePodDelete(context.TODO(), linuxKey))

	winKey, winPod := podOnNode("win-pod", "win-node", "10.244.1.57")
	require.NoError(t, r.ReconcilePod(context.TODO(), winKey, winPod))
	cep := getAndAssertCEPExists(t, ciliumEndpoints, winPod)
	require.Contains(t, cep.Status.Identity.Labels, "cidr:10.244.1.57/32")
	require.Contains(t, cep.Status.Identity.Labels, "k8s:k1=v1")
	require.NotContains(t, cep.Status.Identity.Labels, "reserved:world")

	// Reconciling again with unchanged labels must not allocate a new identity.
	id := cep.Status.Identity.ID
	require.NoError(t, r.ReconcilePod(context.TODO(), winKey, winPod))
	require.Equal(t, id, getAndAssertCEPExists(t, ciliumEndpoints, winPod).Status.Identity.ID)
}

func TestWindowsCIDRLabelsWithoutScoping(t *testing.T) {
	r, ciliumEndpoints := newTestEndpointReconciler(t)
	r.cfg = Config{WindowsCIDRLabels: true}
	createNamespace(r.ciliumSlimClientSet.CoreV1())
	createNode(t, r.ciliumSlimClientSet.CoreV1(), "linux-node", "linux")

	linuxKey, linuxPod := podOnNode("linux-pod", "linux-node", "10.244.0.5")
	require.NoError(t, r.ReconcilePod(context.TODO(), linuxKey, linuxPod))
	cep := getAndAssertCEPExists(t, ciliumEndpoints, linuxPod)
	for _, l := range cep.Status.Identity.Labels {
		require.NotContains(t, l, "cidr:", "linux pods keep upstream CIDR semantics")
	}
}

func TestDefaultConfigUnchanged(t *testing.T) {
	r, ciliumEndpoints := newTestEndpointReconciler(t)
	createNamespace(r.ciliumSlimClientSet.CoreV1())

	// No node objects exist: the default config must not look up node OS.
	winKey, winPod := podOnNode("win-pod", "win-node", "10.244.1.57")
	require.NoError(t, r.ReconcilePod(context.TODO(), winKey, winPod))
	cep := getAndAssertCEPExists(t, ciliumEndpoints, winPod)
	for _, l := range cep.Status.Identity.Labels {
		require.NotContains(t, l, "cidr:")
	}
}

const (
	testIPv4 = "10.244.1.57"
	testIPv6 = "fd00:10:244:1::39"
)

func withPodIPs(pod *v1.Pod, ips ...string) {
	pod.Status.PodIPs = nil
	for _, ip := range ips {
		pod.Status.PodIPs = append(pod.Status.PodIPs, v1.PodIP{IP: ip})
	}
	if len(ips) > 0 {
		pod.Status.PodIP = ips[0]
	}
}

func TestPodDualStackAddressing(t *testing.T) {
	tests := []struct {
		name     string
		podIP    string
		podIPs   []string
		wantIPv4 string
		wantIPv6 string
	}{
		{name: "ipv4 PodIP only", podIP: testIPv4, wantIPv4: testIPv4},
		{name: "ipv4 PodIPs", podIPs: []string{testIPv4}, wantIPv4: testIPv4},
		{name: "dual-stack ipv4 first", podIPs: []string{testIPv4, testIPv6}, wantIPv4: testIPv4, wantIPv6: testIPv6},
		{name: "dual-stack ipv6 first", podIPs: []string{testIPv6, testIPv4}, wantIPv4: testIPv4, wantIPv6: testIPv6},
		{name: "ipv6 only", podIPs: []string{testIPv6}, wantIPv6: testIPv6},
		{name: "ipv6 PodIP only", podIP: testIPv6, wantIPv6: testIPv6},
		{name: "ipv6 canonicalized", podIPs: []string{"FD00:10:244:1:0:0:0:39"}, wantIPv6: testIPv6},
		{name: "first address per family", podIPs: []string{testIPv4, "10.244.1.58", testIPv6, "fd00::1"}, wantIPv4: testIPv4, wantIPv6: testIPv6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, ciliumEndpoints := newTestEndpointReconciler(t)
			createNamespace(r.ciliumSlimClientSet.CoreV1())
			podKey, pod := podTestX()
			pod.Status.PodIP = tt.podIP
			if tt.podIPs != nil {
				withPodIPs(pod, tt.podIPs...)
			}

			require.NoError(t, r.ReconcilePod(context.TODO(), podKey, pod))
			pep, ok := r.store.GetPod(podKey)
			require.True(t, ok)
			require.Equal(t, tt.wantIPv4, pep.ipv4)
			require.Equal(t, tt.wantIPv6, pep.ipv6)

			cep := getAndAssertCEPExists(t, ciliumEndpoints, pod)
			require.Equal(t, ciliumv2.AddressPairList{{IPV4: tt.wantIPv4, IPV6: tt.wantIPv6}}, cep.Status.Networking.Addressing)
			require.Equal(t, []string{"k8s:io.cilium.k8s.policy.cluster", "k8s:io.kubernetes.pod.namespace=test", "k8s:k1=v1"},
				cep.Status.Identity.Labels, "IP addresses do not change identity labels without --windows-cidr-labels")
		})
	}
}

func TestIPv4OnlyCEPStatusUnchanged(t *testing.T) {
	r, ciliumEndpoints := newTestEndpointReconciler(t)
	createNamespace(r.ciliumSlimClientSet.CoreV1())
	podKey, pod := podTestX()
	withPodIPs(pod, "1.2.3.4")
	require.NoError(t, r.ReconcilePod(context.TODO(), podKey, pod))

	b, err := json.Marshal(getAndAssertCEPExists(t, ciliumEndpoints, pod).Status.Networking)
	require.NoError(t, err)
	require.JSONEq(t, `{"addressing":[{"ipv4":"1.2.3.4"}],"node":"10.0.0.1"}`, string(b))
}

func TestPodWithUnparsableIPs(t *testing.T) {
	r, ciliumEndpoints := newTestEndpointReconciler(t)
	createNamespace(r.ciliumSlimClientSet.CoreV1())
	podKey, pod := podTestX()
	withPodIPs(pod, "not-an-ip")
	require.NoError(t, r.ReconcilePod(context.TODO(), podKey, pod))
	assertCEPDoesNotExist(t, ciliumEndpoints, podKey)
}

// TestExistingCEPAddressing covers CEPs found in the store at startup (not in the cache).
func TestExistingCEPAddressing(t *testing.T) {
	const existingEndpointID = 7
	tests := []struct {
		name           string
		addressing     ciliumv2.AddressPairList
		podIPs         []string
		wantEndpointID int64
	}{
		{
			name:           "ipv6 only, unchanged",
			addressing:     ciliumv2.AddressPairList{{IPV6: testIPv6}},
			podIPs:         []string{testIPv6},
			wantEndpointID: existingEndpointID,
		},
		{
			name:           "dual-stack, unchanged",
			addressing:     ciliumv2.AddressPairList{{IPV4: testIPv4, IPV6: testIPv6}},
			podIPs:         []string{testIPv4, testIPv6},
			wantEndpointID: existingEndpointID,
		},
		{
			name:           "dual-stack in separate pairs, unchanged",
			addressing:     ciliumv2.AddressPairList{{IPV6: testIPv6}, {IPV4: testIPv4}},
			podIPs:         []string{testIPv4, testIPv6},
			wantEndpointID: existingEndpointID,
		},
		{
			name:           "ipv4 only, pod gained ipv6",
			addressing:     ciliumv2.AddressPairList{{IPV4: testIPv4}},
			podIPs:         []string{testIPv4, testIPv6},
			wantEndpointID: existingEndpointID + 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, ciliumEndpoints := newTestEndpointReconciler(t)
			createNamespace(r.ciliumSlimClientSet.CoreV1())
			podKey, pod := podTestX()
			withPodIPs(pod, tt.podIPs...)

			ciliumEndpoints.Upsert(&ciliumv2.CiliumEndpoint{
				ObjectMeta: metav1.ObjectMeta{Name: podKey.Name, Namespace: podKey.Namespace},
				Status: ciliumv2.EndpointStatus{
					ID: existingEndpointID,
					Identity: &ciliumv2.EndpointIdentity{
						ID:     12345,
						Labels: []string{"k8s:io.cilium.k8s.policy.cluster", "k8s:io.kubernetes.pod.namespace=test", "k8s:k1=v1"},
					},
					Networking: &ciliumv2.EndpointNetworking{NodeIP: "10.0.0.1", Addressing: tt.addressing},
					State:      "ready",
				},
			})

			require.NoError(t, r.ReconcilePod(context.TODO(), podKey, pod))
			pep, ok := r.store.GetPod(podKey)
			require.True(t, ok)
			require.Equal(t, tt.wantEndpointID, pep.endpointID)
			require.Equal(t, testIPv6, pep.ipv6)
		})
	}
}

func TestPodIPv6Changed(t *testing.T) {
	r, ciliumEndpoints := newTestEndpointReconciler(t)
	createNamespace(r.ciliumSlimClientSet.CoreV1())
	podKey, pod := podTestX()
	withPodIPs(pod, testIPv4, testIPv6)
	require.NoError(t, r.ReconcilePod(context.TODO(), podKey, pod))
	pep, _ := r.store.GetPod(podKey)
	identityID := pep.identityID
	endpointID := pep.endpointID

	steps := []struct {
		name   string
		podIPs []string
		want   ciliumv2.AddressPairList
	}{
		{"ipv6 changed", []string{testIPv4, "fd00:10:244:1::40"}, ciliumv2.AddressPairList{{IPV4: testIPv4, IPV6: "fd00:10:244:1::40"}}},
		{"ipv6 removed", []string{testIPv4}, ciliumv2.AddressPairList{{IPV4: testIPv4}}},
		{"ipv6 added", []string{testIPv4, testIPv6}, ciliumv2.AddressPairList{{IPV4: testIPv4, IPV6: testIPv6}}},
	}
	for _, step := range steps {
		_, pod = podTestX()
		withPodIPs(pod, step.podIPs...)
		require.NoError(t, r.ReconcilePod(context.TODO(), podKey, pod), step.name)
		endpointID++
		cep := getAndAssertCEPExists(t, ciliumEndpoints, pod)
		require.Equal(t, step.want, cep.Status.Networking.Addressing, step.name)
		require.Equal(t, endpointID, cep.Status.ID, step.name)
		require.Equal(t, identityID, cep.Status.Identity.ID, "%s: identity unchanged without cidr labels", step.name)
	}
}

func TestWindowsCIDRLabelsDualStack(t *testing.T) {
	r, ciliumEndpoints := newTestEndpointReconciler(t)
	r.cfg = Config{WindowsOnlyEndpoints: true, WindowsCIDRLabels: true}
	createNamespace(r.ciliumSlimClientSet.CoreV1())
	createNode(t, r.ciliumSlimClientSet.CoreV1(), "win-node", "windows")

	// Cilium keys IPv6 cidr labels with ":" replaced by "-" (and a "0" instead of a leading or
	// trailing "-"), but prints them with colons in CiliumEndpoint/CiliumIdentity label lists.
	tests := []struct {
		name      string
		podIPs    []string
		wantKeys  []string
		wantModel []string
	}{
		{"ipv4 only", []string{testIPv4}, []string{"10.244.1.57/32"}, []string{"cidr:10.244.1.57/32"}},
		{
			name:      "dual-stack",
			podIPs:    []string{testIPv4, testIPv6},
			wantKeys:  []string{"10.244.1.57/32", "fd00-10-244-1--39/128"},
			wantModel: []string{"cidr:10.244.1.57/32", "cidr:fd00:10:244:1::39/128"},
		},
		{"ipv6 only", []string{testIPv6}, []string{"fd00-10-244-1--39/128"}, []string{"cidr:fd00:10:244:1::39/128"}},
		{"leading ::", []string{"::a"}, []string{"0--a/128"}, []string{"cidr:::a/128"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			winKey, winPod := podOnNode("win-"+strings.ReplaceAll(tt.name, " ", "-"), "win-node", "")
			withPodIPs(winPod, tt.podIPs...)
			require.NoError(t, r.ReconcilePod(context.TODO(), winKey, winPod))
			pep, ok := r.store.GetPod(winKey)
			require.True(t, ok)

			var cidrKeys []string
			for k, l := range pep.lbls {
				if l.Source == labels.LabelSourceCIDR {
					cidrKeys = append(cidrKeys, k)
				}
			}
			require.ElementsMatch(t, tt.wantKeys, cidrKeys)

			cep := getAndAssertCEPExists(t, ciliumEndpoints, winPod)
			var cidrModel []string
			for _, l := range cep.Status.Identity.Labels {
				if strings.HasPrefix(l, labels.LabelSourceCIDR+":") {
					cidrModel = append(cidrModel, l)
				}
			}
			require.ElementsMatch(t, tt.wantModel, cidrModel)
			require.NotContains(t, cep.Status.Identity.Labels, "reserved:world")
		})
	}
}

// TestExistingCEPCIDRLabelsEqual checks that unchanged cidr labels read back from a CiliumEndpoint
// at startup compare equal, so the CiliumEndpoint is not patched.
func TestExistingCEPCIDRLabelsEqual(t *testing.T) {
	const existingIdentityID = 12345
	tests := []struct {
		name   string
		podIPs []string
	}{
		{"ipv4", []string{testIPv4}},
		{"dual-stack", []string{testIPv4, testIPv6}},
		{"ipv6", []string{testIPv6}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, ciliumEndpoints := newTestEndpointReconciler(t)
			r.cfg = Config{WindowsOnlyEndpoints: true, WindowsCIDRLabels: true}
			createNamespace(r.ciliumSlimClientSet.CoreV1())
			createNode(t, r.ciliumSlimClientSet.CoreV1(), "win-node", "windows")
			podKey, pod := podOnNode("win-pod", "win-node", "")
			withPodIPs(pod, tt.podIPs...)

			r.Lock()
			lbls, err := r.ciliumEndpointsLabels(context.TODO(), pod)
			r.Unlock()
			require.NoError(t, err)
			ipv4, ipv6 := podAddrs(pod)
			ciliumEndpoints.Upsert(&ciliumv2.CiliumEndpoint{
				ObjectMeta: metav1.ObjectMeta{Name: podKey.Name, Namespace: podKey.Namespace},
				Status: (&PodEndpoint{
					endpointID: 1,
					identityID: existingIdentityID,
					lbls:       lbls,
					ipv4:       addrString(ipv4),
					ipv6:       addrString(ipv6),
					nodeIP:     pod.Status.HostIP,
				}).endpointStatus(),
			})

			require.NoError(t, r.ReconcilePod(context.TODO(), podKey, pod))
			require.Equal(t, int64(existingIdentityID), getAndAssertCEPExists(t, ciliumEndpoints, pod).Status.Identity.ID,
				"CiliumEndpoint must not be patched")
		})
	}
}
