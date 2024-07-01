package endpointcontroller

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	ciliumv2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	ciliumclient "github.com/cilium/cilium/pkg/k8s/client"
	"github.com/cilium/cilium/pkg/k8s/resource"
	v1 "github.com/cilium/cilium/pkg/k8s/slim/k8s/api/core/v1"
	slim_metav1 "github.com/cilium/cilium/pkg/k8s/slim/k8s/apis/meta/v1"
	corev1 "github.com/cilium/cilium/pkg/k8s/slim/k8s/client/clientset/versioned/typed/core/v1"
	"github.com/cilium/cilium/pkg/labels"
	"github.com/cilium/cilium/pkg/option"
	"github.com/microsoft/retina/pkg/controllers/operator/cilium-crds/cache"
	ciliumutil "github.com/microsoft/retina/pkg/utils/testutil/cilium"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	l := logrus.New()
	l.SetLevel(logrus.DebugLevel)
	ciliumEndpoints := ciliumutil.NewMockResource[*ciliumv2.CiliumEndpoint](l)

	fakeClientSet, _ := ciliumclient.NewFakeClientset()

	m := ciliumutil.NewMockVersionedClient(l, ciliumEndpoints)
	r := &endpointReconciler{
		l:                   l,
		clientset:           m,
		podEvents:           nil,
		ciliumEndpoints:     ciliumEndpoints,
		ciliumSlimClientSet: fakeClientSet.SlimFakeClientset,
		store:               NewStore(),
		Mutex:               &sync.Mutex{},
	}

	// make sure to use CRD mode (this is referenced in InitIdentityAllocator)
	option.Config.IdentityAllocationMode = option.IdentityAllocationModeCRD
	im, err := NewIdentityManager(l, m)
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
