// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package endpointcontroller

import (
	"context"
	"log/slog"
	"testing"
	"time"

	identitykey "github.com/cilium/cilium/pkg/identity/key"
	"github.com/cilium/cilium/pkg/k8s"
	ciliumv2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	ciliumclient "github.com/cilium/cilium/pkg/k8s/client/testutils"
	v1 "github.com/cilium/cilium/pkg/k8s/slim/k8s/api/core/v1"
	slim_metav1 "github.com/cilium/cilium/pkg/k8s/slim/k8s/apis/meta/v1"
	"github.com/cilium/cilium/pkg/labels"
	"github.com/cilium/cilium/pkg/labelsfilter"
	"github.com/cilium/cilium/pkg/option"
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
	setClusterName(t, "aks-prod")
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
			require.Equal(t, "aks-prod", lbls["io.cilium.k8s.policy.cluster"].Value)
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

func createCiliumConfig(t *testing.T, r *endpointReconciler, data map[string]string) {
	t.Helper()
	_, err := r.k8sClient.CoreV1().ConfigMaps(ciliumConfigNamespace).Create(context.TODO(), &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: ciliumConfigName, Namespace: ciliumConfigNamespace},
		Data:       data,
	}, metav1.CreateOptions{})
	require.NoError(t, err)
}

func TestReuseOperatorIdentitiesStartupValidation(t *testing.T) {
	t.Run("incompatible with windows-cidr-labels", func(t *testing.T) {
		r, _ := newTestEndpointReconciler(t)
		r.identityManager = nil
		r.cfg = Config{ReuseOperatorIdentities: true, WindowsCIDRLabels: true}
		createCiliumConfig(t, r, map[string]string{ciliumConfigIdentityManagementMode: option.IdentityManagementModeOperator})

		err := r.Start(context.TODO())
		require.ErrorIs(t, err, ErrIncompatibleFlags)
		require.Nil(t, r.identityManager)
		require.Nil(t, r.wp, "no workers may start")
	})

	t.Run("incompatible flags rejected at registration, before leader election", func(t *testing.T) {
		fakeClientSet, _ := ciliumclient.NewFakeClientset(slog.Default())
		require.ErrorIs(t, registerEndpointController(params{
			Clientset: fakeClientSet,
			Config:    Config{ReuseOperatorIdentities: true, WindowsCIDRLabels: true},
		}), ErrIncompatibleFlags)
	})

	t.Run("missing cilium-config", func(t *testing.T) {
		r, _ := newTestEndpointReconciler(t)
		r.cfg = Config{ReuseOperatorIdentities: true}
		require.Error(t, r.setupOperatorIdentities(context.TODO()))
	})

	t.Run("identity-management-mode not operator", func(t *testing.T) {
		r, _ := newTestEndpointReconciler(t)
		r.cfg = Config{ReuseOperatorIdentities: true}
		createCiliumConfig(t, r, map[string]string{ciliumConfigIdentityManagementMode: "agent"})
		require.ErrorIs(t, r.setupOperatorIdentities(context.TODO()), ErrIdentityModeNotOperator)
	})

	t.Run("invalid labels", func(t *testing.T) {
		useDefaultLabelFilter(t)
		setClusterName(t, "")
		r, _ := newTestEndpointReconciler(t)
		r.cfg = Config{ReuseOperatorIdentities: true}
		createCiliumConfig(t, r, map[string]string{
			ciliumConfigIdentityManagementMode: option.IdentityManagementModeOperator,
			ciliumConfigLabels:                 "k8s:app k8s:[",
		})
		require.Error(t, r.setupOperatorIdentities(context.TODO()))
		require.Empty(t, option.Config.ClusterName, "nothing is changed on error")
	})

	t.Run("sets cluster name and label filter", func(t *testing.T) {
		useDefaultLabelFilter(t)
		setClusterName(t, "default")
		r, _ := newTestEndpointReconciler(t)
		r.cfg = Config{ReuseOperatorIdentities: true}
		createCiliumConfig(t, r, map[string]string{
			ciliumConfigIdentityManagementMode: option.IdentityManagementModeOperator,
			ciliumConfigClusterName:            "aks-prod",
			ciliumConfigLabels:                 "k8s:app  k8s:io.kubernetes.pod.namespace",
		})
		require.NoError(t, r.setupOperatorIdentities(context.TODO()))
		require.Equal(t, "aks-prod", option.Config.ClusterName)

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

	// No identity exists for these labels and cilium-operator is not running: lookup times out without creating one.
	ctx, cancel := context.WithTimeout(context.TODO(), 2*time.Second)
	defer cancel()
	_, err = im.GetIdentityAndIncrementReference(ctx,
		labels.Map2Labels(map[string]string{testAppKey: testOtherApp, testNamespaceLabel: testNamespace}, labels.LabelSourceK8s))
	require.Error(t, err)

	list, err := cids.List(context.TODO(), metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, list.Items, 1, "operator-managed mode must not create CiliumIdentities")

	im.DecrementReference(context.TODO(), lbls)
	require.Empty(t, im.labelIdentities)
	_, err = cids.Get(context.TODO(), "21279", metav1.GetOptions{})
	require.NoError(t, err)
}
