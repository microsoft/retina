// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package endpointcontroller

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	identitykey "github.com/cilium/cilium/pkg/identity/key"
	"github.com/cilium/cilium/pkg/k8s"
	ciliumv2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	ciliumclient "github.com/cilium/cilium/pkg/k8s/client/testutils"
	"github.com/cilium/cilium/pkg/k8s/identitybackend"
	v1 "github.com/cilium/cilium/pkg/k8s/slim/k8s/api/core/v1"
	slim_metav1 "github.com/cilium/cilium/pkg/k8s/slim/k8s/apis/meta/v1"
	"github.com/cilium/cilium/pkg/labels"
	"github.com/cilium/cilium/pkg/labelsfilter"
	"github.com/cilium/cilium/pkg/option"
	ciliumutil "github.com/microsoft/retina/pkg/utils/testutil/cilium"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// useDefaultLabelFilter configures the process-global label filter; callers must not run in parallel.
const (
	testNamespaceLabel    = "io.kubernetes.pod.namespace"
	testK8sNamespaceLabel = "k8s:" + testNamespaceLabel
	testNamespace         = "prod"
	testAppKey            = "app"
	testOtherApp          = "other"
	testWebApp            = "web"
	testClusterName       = "aks-prod"
)

func useDefaultLabelFilter(t *testing.T) {
	t.Helper()
	l := slog.Default()
	require.NoError(t, labelsfilter.ParseLabelPrefixCfg(l, nil, nil, ""))
	t.Cleanup(func() { _ = labelsfilter.ParseLabelPrefixCfg(l, nil, nil, "") })
}

func setClusterName(t *testing.T, name string) {
	t.Helper()
	prev := option.Config.ClusterName
	option.Config.ClusterName = name
	t.Cleanup(func() { option.Config.ClusterName = prev })
}

func operatorTestPods() map[string]*v1.Pod {
	pod := func(name string, lbls map[string]string) *v1.Pod {
		return &v1.Pod{
			ObjectMeta: slim_metav1.ObjectMeta{Name: name, Namespace: testNamespace, UID: types.UID("uid-" + name), Labels: lbls},
			Spec:       v1.PodSpec{ServiceAccountName: "web-sa"},
			Status:     v1.PodStatus{PodIP: "10.244.1.57"},
		}
	}
	return map[string]*v1.Pod{
		"bare": pod("bare", map[string]string{testAppKey: testWebApp}),
		"deployment": pod("deploy-7d4b9c-x2x", map[string]string{
			testAppKey: testWebApp, "pod-template-hash": "7d4b9c",
		}),
		"daemonset": pod("ds-abcde", map[string]string{
			testAppKey: testWebApp, "controller-revision-hash": "6f7c8d", "pod-template-generation": "3",
		}),
	}
}

func newOperatorLabelsTestReconciler(t *testing.T, reuse bool) (*endpointReconciler, *v1.Namespace) {
	t.Helper()
	r, _ := newTestEndpointReconciler(t)
	r.cfg = Config{ReuseOperatorIdentities: reuse}
	ns := &v1.Namespace{ObjectMeta: slim_metav1.ObjectMeta{Name: testNamespace, Labels: map[string]string{"team": "payments"}}}
	_, err := r.ciliumSlimClientSet.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
	require.NoError(t, err)
	return r, ns
}

var hashLabels = []string{"pod-template-hash", "controller-revision-hash", "pod-template-generation"}

func TestReuseOperatorIdentitiesKeyParity(t *testing.T) {
	useDefaultLabelFilter(t)
	setClusterName(t, testClusterName)
	r, ns := newOperatorLabelsTestReconciler(t, true)

	for name, pod := range operatorTestPods() {
		t.Run(name, func(t *testing.T) {
			lbls, err := r.ciliumEndpointsLabels(context.TODO(), pod)
			require.NoError(t, err)

			// cilium-operator's CID controller (operator/pkg/ciliumidentity allocateCIDForPod).
			_, podLabels := k8s.GetPodMetadata(slog.Default(), ns, pod)
			operatorKey := identitykey.GetCIDKeyFromLabels(podLabels, labels.LabelSourceK8s).GetKey()
			// CachingIdentityAllocator.AllocateIdentity builds its key this way.
			retinaKey := (&identitykey.GlobalIdentity{LabelArray: lbls.LabelArray()}).GetKey()
			require.Equal(t, operatorKey, retinaKey)

			for _, h := range hashLabels {
				require.NotContains(t, lbls, h)
			}
			require.Equal(t, testClusterName, lbls["io.cilium.k8s.policy.cluster"].Value)
			require.Equal(t, "web-sa", lbls["io.cilium.k8s.policy.serviceaccount"].Value)
			require.Equal(t, "payments", lbls["io.cilium.k8s.namespace.labels.team"].Value)
			require.Equal(t, testWebApp, lbls[testAppKey].Value)
		})
	}
}

func TestReuseOperatorIdentitiesOffKeepsLabels(t *testing.T) {
	r, _ := newOperatorLabelsTestReconciler(t, false)

	for name, pod := range operatorTestPods() {
		lbls, err := r.ciliumEndpointsLabels(context.TODO(), pod)
		require.NoError(t, err, name)
		for k := range pod.Labels {
			require.Contains(t, lbls, k, name)
		}
	}
}

func shortenSetupBackoff(t *testing.T) {
	t.Helper()
	prevInitial, prevMax := operatorIdentitiesSetupInitialBackoff, operatorIdentitiesSetupMaxBackoff
	operatorIdentitiesSetupInitialBackoff, operatorIdentitiesSetupMaxBackoff = time.Millisecond, 5*time.Millisecond
	t.Cleanup(func() {
		operatorIdentitiesSetupInitialBackoff, operatorIdentitiesSetupMaxBackoff = prevInitial, prevMax
	})
}

func waitSetupDone(t *testing.T, r *endpointReconciler) {
	t.Helper()
	select {
	case <-r.setupDone:
	case <-time.After(10 * time.Second):
		t.Fatal("operator identities setup did not finish")
	}
}

func createCiliumConfig(t *testing.T, r *endpointReconciler, data map[string]string) {
	t.Helper()
	_, err := r.k8sClient.CoreV1().ConfigMaps(ciliumConfigNamespace).Create(context.TODO(), &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: ciliumConfigName, Namespace: ciliumConfigNamespace},
		Data:       data,
	}, metav1.CreateOptions{})
	require.NoError(t, err)
}

func TestReuseOperatorIdentitiesStartupValidation(t *testing.T) {
	t.Run("missing cilium-config", func(t *testing.T) {
		r, _ := newTestEndpointReconciler(t)
		r.cfg = Config{ReuseOperatorIdentities: true}
		require.Error(t, r.setupOperatorIdentities(context.TODO()))
	})

	invalid := []struct {
		name    string
		data    map[string]string
		wantErr error
	}{
		{
			name:    "identity-management-mode not operator",
			data:    map[string]string{ciliumConfigIdentityManagementMode: "agent"},
			wantErr: ErrIdentityModeNotOperator,
		},
		{
			name: "label-prefix-file rejected",
			data: map[string]string{
				ciliumConfigIdentityManagementMode: option.IdentityManagementModeOperator,
				ciliumConfigLabelPrefixFile:        "/etc/cilium/labels.json",
			},
			wantErr: ErrLabelPrefixFileUnsupported,
		},
		{
			name: "invalid labels",
			data: map[string]string{
				ciliumConfigIdentityManagementMode: option.IdentityManagementModeOperator,
				ciliumConfigLabels:                 "k8s:app k8s:[",
			},
			wantErr: ErrInvalidLabelPrefixCfg,
		},
	}
	for _, tt := range invalid {
		t.Run(tt.name, func(t *testing.T) {
			useDefaultLabelFilter(t)
			setClusterName(t, "")
			r, _ := newTestEndpointReconciler(t)
			r.cfg = Config{ReuseOperatorIdentities: true}
			createCiliumConfig(t, r, tt.data)
			require.ErrorIs(t, r.setupOperatorIdentities(context.TODO()), tt.wantErr)
			require.Empty(t, option.Config.ClusterName, "nothing is changed on error")
		})
	}

	t.Run("invalid cilium-config degrades instead of failing Start", func(t *testing.T) {
		shortenSetupBackoff(t)
		r, _ := newTestEndpointReconciler(t)
		r.identityManager = nil
		r.cfg = Config{ReuseOperatorIdentities: true}
		createCiliumConfig(t, r, map[string]string{
			ciliumConfigIdentityManagementMode: option.IdentityManagementModeOperator,
			ciliumConfigLabelPrefixFile:        "/etc/cilium/labels.json",
		})

		require.NoError(t, r.Start(context.TODO()))
		time.Sleep(50 * time.Millisecond)
		select {
		case <-r.setupDone:
			t.Fatal("setup must keep retrying while cilium-config is invalid")
		default:
		}

		require.NoError(t, r.Stop(context.TODO()))
		waitSetupDone(t, r)
		require.Nil(t, r.identityManager, "no controllers may start")
	})

	t.Run("recovers once cilium-config becomes valid", func(t *testing.T) {
		shortenSetupBackoff(t)
		useDefaultLabelFilter(t)
		setClusterName(t, "")
		r, _ := newTestEndpointReconciler(t)
		r.identityManager = nil
		r.cfg = Config{ReuseOperatorIdentities: true}
		l := slog.Default()
		r.pods = ciliumutil.NewMockResource[*v1.Pod](l)
		r.namespaces = ciliumutil.NewMockResource[*v1.Namespace](l)

		require.NoError(t, r.Start(context.TODO()))
		t.Cleanup(func() { _ = r.Stop(context.TODO()) })
		time.Sleep(20 * time.Millisecond)
		createCiliumConfig(t, r, map[string]string{
			ciliumConfigIdentityManagementMode: option.IdentityManagementModeOperator,
			ciliumConfigClusterName:            testClusterName,
		})

		waitSetupDone(t, r)
		require.NotNil(t, r.identityManager)
		require.Equal(t, testClusterName, option.Config.ClusterName)
	})

	t.Run("Stop before Start", func(t *testing.T) {
		r := &endpointReconciler{}
		require.NotPanics(t, func() { require.NoError(t, r.Stop(context.TODO())) })
	})

	t.Run("sets cluster name and label filter", func(t *testing.T) {
		useDefaultLabelFilter(t)
		setClusterName(t, "default")
		r, _ := newTestEndpointReconciler(t)
		r.cfg = Config{ReuseOperatorIdentities: true}
		createCiliumConfig(t, r, map[string]string{
			ciliumConfigIdentityManagementMode: option.IdentityManagementModeOperator,
			ciliumConfigClusterName:            testClusterName,
			ciliumConfigLabels:                 "k8s:app  k8s:io.kubernetes.pod.namespace",
		})
		require.NoError(t, r.setupOperatorIdentities(context.TODO()))
		require.Equal(t, testClusterName, option.Config.ClusterName)

		identityLabels, _ := labelsfilter.Filter(labels.Map2Labels(map[string]string{
			testAppKey: testWebApp, "team": "payments", testNamespaceLabel: testNamespace,
		}, labels.LabelSourceK8s))
		require.Equal(t, "k8s:app=web,k8s:io.kubernetes.pod.namespace=prod", identityLabels.String())
	})
}

func TestOperatorManagedIdentityManagerOnlyLooksUp(t *testing.T) {
	l := slog.Default()
	fakeClientSet, _ := ciliumclient.NewFakeClientset(l)
	option.Config.IdentityAllocationMode = option.IdentityAllocationModeCRD

	cids := fakeClientSet.CiliumFakeClientset.CiliumV2().CiliumIdentities()
	_, err := cids.Create(context.TODO(), &ciliumv2.CiliumIdentity{
		ObjectMeta: metav1.ObjectMeta{Name: "21279"},
		SecurityLabels: map[string]string{
			"k8s:app":             testWebApp,
			testK8sNamespaceLabel: testNamespace,
		},
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	im, err := NewIdentityManager(context.TODO(), l, fakeClientSet.CiliumFakeClientset, true)
	require.NoError(t, err)

	lbls := labels.Map2Labels(map[string]string{testAppKey: testWebApp, testNamespaceLabel: testNamespace}, labels.LabelSourceK8s)
	id, err := im.GetIdentityAndIncrementReference(context.TODO(), lbls)
	require.NoError(t, err)
	require.Equal(t, int64(21279), id)

	// No identity exists for these labels and cilium-operator is not running: lookup fails fast without creating one.
	start := time.Now()
	_, err = im.GetIdentityAndIncrementReference(context.TODO(),
		labels.Map2Labels(map[string]string{testAppKey: testOtherApp, testNamespaceLabel: testNamespace}, labels.LabelSourceK8s))
	require.ErrorIs(t, err, ErrIdentityNotFound)
	require.Less(t, time.Since(start), 2*time.Second, "lookup must not block waiting for cilium-operator")

	list, err := cids.List(context.TODO(), metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, list.Items, 1, "operator-managed mode must not create CiliumIdentities")

	im.DecrementReference(context.TODO(), lbls)
	require.Empty(t, im.labelIdentities)
	_, err = cids.Get(context.TODO(), "21279", metav1.GetOptions{})
	require.NoError(t, err)
}

func TestOperatorManagedIdentityReconcileAfterIdentityCreated(t *testing.T) {
	useDefaultLabelFilter(t)
	setClusterName(t, "default")

	l := slog.Default()
	ciliumEndpoints := ciliumutil.NewMockResource[*ciliumv2.CiliumEndpoint](l)
	fakeClientSet, _ := ciliumclient.NewFakeClientset(l)
	r := &endpointReconciler{
		l:                   l,
		clientset:           ciliumutil.NewMockVersionedClient(l, ciliumEndpoints),
		ciliumEndpoints:     ciliumEndpoints,
		ciliumSlimClientSet: fakeClientSet.SlimFakeClientset,
		k8sClient:           fakeClientSet,
		store:               NewStore(),
		Mutex:               &sync.Mutex{},
		windowsNodes:        make(map[string]bool),
		cfg:                 Config{WindowsOnlyEndpoints: true},
	}
	option.Config.IdentityAllocationMode = option.IdentityAllocationModeCRD
	im, err := NewIdentityManager(context.TODO(), l, fakeClientSet.CiliumFakeClientset, true)
	require.NoError(t, err)
	r.identityManager = im

	createNamespace(r.ciliumSlimClientSet.CoreV1())
	createNode(t, r.ciliumSlimClientSet.CoreV1(), "win-node", "windows")
	podKey, pod := podTestX()
	pod.Spec.NodeName = "win-node"

	cids := fakeClientSet.CiliumFakeClientset.CiliumV2().CiliumIdentities()
	countIdentities := func() int {
		list, listErr := cids.List(context.TODO(), metav1.ListOptions{})
		require.NoError(t, listErr)
		return len(list.Items)
	}

	err = r.ReconcilePod(context.TODO(), podKey, pod)
	require.ErrorIs(t, err, ErrIdentityNotFound)
	assertCEPDoesNotExist(t, ciliumEndpoints, podKey)
	require.Equal(t, 0, countIdentities())

	podLabels, err := r.ciliumEndpointsLabels(context.TODO(), pod)
	require.NoError(t, err)
	securityLabels := make(map[string]string, len(podLabels))
	for _, lbl := range podLabels {
		securityLabels[lbl.Source+":"+lbl.Key] = lbl.Value
	}
	_, err = cids.Create(context.TODO(), &ciliumv2.CiliumIdentity{
		ObjectMeta:     metav1.ObjectMeta{Name: "12345"},
		SecurityLabels: securityLabels,
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return r.ReconcilePod(context.TODO(), podKey, pod) == nil
	}, 10*time.Second, 50*time.Millisecond)

	cep := getAndAssertCEPExists(t, ciliumEndpoints, pod)
	require.NotNil(t, cep.Status.Identity)
	require.Equal(t, int64(12345), cep.Status.Identity.ID)
	require.Equal(t, 1, countIdentities())
}

func TestCombinedModeDualStackReusesOperatorIdentity(t *testing.T) {
	useDefaultLabelFilter(t)
	setClusterName(t, "")

	l := slog.Default()
	ciliumEndpoints := ciliumutil.NewMockResource[*ciliumv2.CiliumEndpoint](l)
	fakeClientSet, _ := ciliumclient.NewFakeClientset(l)
	r := &endpointReconciler{
		l:                   l,
		clientset:           ciliumutil.NewMockVersionedClient(l, ciliumEndpoints),
		ciliumEndpoints:     ciliumEndpoints,
		ciliumSlimClientSet: fakeClientSet.SlimFakeClientset,
		k8sClient:           fakeClientSet,
		store:               NewStore(),
		Mutex:               &sync.Mutex{},
		windowsNodes:        make(map[string]bool),
		cfg:                 Config{WindowsOnlyEndpoints: true, ReuseOperatorIdentities: true},
	}
	createCiliumConfig(t, r, map[string]string{
		ciliumConfigIdentityManagementMode: option.IdentityManagementModeOperator,
		ciliumConfigClusterName:            testClusterName,
	})
	require.NoError(t, r.setupOperatorIdentities(context.TODO()))

	option.Config.IdentityAllocationMode = option.IdentityAllocationModeCRD
	im, err := NewIdentityManager(context.TODO(), l, fakeClientSet.CiliumFakeClientset, true)
	require.NoError(t, err)
	r.identityManager = im

	coreV1 := r.ciliumSlimClientSet.CoreV1()
	createNamespace(coreV1)
	createNode(t, coreV1, "linux-node", "linux")
	createNode(t, coreV1, "win-node", "windows")

	linuxKey, linuxPod := podOnNode("linux-pod", "linux-node", testIPv4)
	withPodIPs(linuxPod, testIPv4, testIPv6)
	winKey, winPod := podOnNode("win-pod", "win-node", testIPv4)
	withPodIPs(winPod, testIPv4, testIPv6)
	winPod.Labels["pod-template-hash"] = "7d4b9c"

	// Seed the identity as cilium-operator's CID controller would (operator/pkg/ciliumidentity createCID).
	ns, err := coreV1.Namespaces().Get(context.TODO(), winPod.Namespace, metav1.GetOptions{})
	require.NoError(t, err)
	_, podLabels := k8s.GetPodMetadata(l, ns, winPod)
	cidKey := identitykey.GetCIDKeyFromLabels(podLabels, labels.LabelSourceK8s)
	cids := fakeClientSet.CiliumFakeClientset.CiliumV2().CiliumIdentities()
	_, err = cids.Create(context.TODO(), &ciliumv2.CiliumIdentity{
		ObjectMeta:     metav1.ObjectMeta{Name: "12345", Labels: identitybackend.SelectK8sLabels(cidKey.GetAsMap())},
		SecurityLabels: cidKey.GetAsMap(),
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	require.NoError(t, r.ReconcilePod(context.TODO(), linuxKey, linuxPod))
	assertCEPDoesNotExist(t, ciliumEndpoints, linuxKey)

	require.Eventually(t, func() bool {
		return r.ReconcilePod(context.TODO(), winKey, winPod) == nil
	}, 10*time.Second, 50*time.Millisecond)

	cep := getAndAssertCEPExists(t, ciliumEndpoints, winPod)
	require.NotNil(t, cep.Status.Networking)
	require.Equal(t, ciliumv2.AddressPairList{{IPV4: testIPv4, IPV6: testIPv6}}, cep.Status.Networking.Addressing)
	require.NotNil(t, cep.Status.Identity)
	require.Equal(t, int64(12345), cep.Status.Identity.ID)
	require.NotContains(t, cep.Status.Identity.Labels, "k8s:pod-template-hash=7d4b9c")

	list, err := cids.List(context.TODO(), metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, list.Items, 1, "no new CiliumIdentity may be created")
}
