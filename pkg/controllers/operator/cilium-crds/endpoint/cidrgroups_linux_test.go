// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package endpointcontroller

import (
	"context"
	"log/slog"
	"net/netip"
	"testing"

	ciliumv2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	slim_metav1 "github.com/cilium/cilium/pkg/k8s/slim/k8s/apis/meta/v1"
	"github.com/cilium/cilium/pkg/labels"
	"github.com/cilium/cilium/pkg/option"
	"github.com/cilium/cilium/pkg/policy/api"
	policytypes "github.com/cilium/cilium/pkg/policy/types"
	ciliumutil "github.com/microsoft/retina/pkg/utils/testutil/cilium"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const teamLabel = "team"

func cidrGroup(name string, lbls map[string]string, cidrs ...string) *ciliumv2.CiliumCIDRGroup {
	g := &ciliumv2.CiliumCIDRGroup{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: lbls}}
	for _, c := range cidrs {
		g.Spec.ExternalCIDRs = append(g.Spec.ExternalCIDRs, api.CIDR(c))
	}
	return g
}

func newCIDRGroupTestReconciler(t *testing.T, cfg Config) *endpointReconciler {
	t.Helper()
	r, _ := newTestEndpointReconciler(t)
	r.cfg = cfg
	r.cidrGroups = ciliumutil.NewMockResource[*ciliumv2.CiliumCIDRGroup](slog.Default())
	r.cidrGroupCache = newCIDRGroupCache()
	createNamespace(r.ciliumSlimClientSet.CoreV1())
	createNode(t, r.ciliumSlimClientSet.CoreV1(), "linux-node", "linux")
	createNode(t, r.ciliumSlimClientSet.CoreV1(), "win-node", "windows")
	return r
}

func TestCIDRGroupLabelsFor(t *testing.T) {
	c := newCIDRGroupCache()
	c.upsert(cidrGroup("a", map[string]string{teamLabel: "x"}, "10.244.1.0/24", "not-a-cidr"))
	c.upsert(cidrGroup("b", nil, "10.244.1.57/32"))
	c.upsert(cidrGroup("c", nil, "192.168.0.0/16"))

	tests := []struct {
		ip   string
		want []string
	}{
		{"10.244.2.1", nil},
		{"10.244.1.9", []string{"a"}},
		{"10.244.1.57", []string{"a", "b"}},
	}
	for _, tt := range tests {
		lbls := c.labelsFor(netip.MustParseAddr(tt.ip))
		for _, name := range []string{"a", "b", "c"} {
			_, ok := lbls[api.LabelForCIDRGroupRef(name).Key]
			require.Equal(t, contains(tt.want, name), ok, "ip %s group %s", tt.ip, name)
		}
		if contains(tt.want, "a") {
			require.Contains(t, lbls, teamLabel, "plain group label for Exists selectors")
			require.Contains(t, lbls, teamLabel+labels.CIDRGroupEncodedSep+"x", "encoded group label")
			require.Equal(t, labels.LabelSourceCIDRGroup, lbls[teamLabel].Source)
		}
	}

	c.delete("a")
	require.NotContains(t, c.labelsFor(netip.MustParseAddr("10.244.1.9")), api.LabelForCIDRGroupRef("a").Key)
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// TestCIDRGroupLabelsMatchPolicySelectors checks the labels against Cilium's own rule selectors.
func TestCIDRGroupLabelsMatchPolicySelectors(t *testing.T) {
	// Cilium >= 1.20 only matches CIDR selectors on non-world identities in "pods" mode.
	saved := option.Config.PolicyCIDRMatchMode
	option.Config.PolicyCIDRMatchMode = []string{"pods"}
	t.Cleanup(func() { option.Config.PolicyCIDRMatchMode = saved })

	r := newCIDRGroupTestReconciler(t, Config{WindowsCIDRLabels: true})
	r.cidrGroupCache.upsert(cidrGroup("win", map[string]string{teamLabel: "x"}, "10.244.1.0/24"))
	_, winPod := podOnNode("win-pod", "win-node", "10.244.1.57")
	r.Lock()
	lbls, err := r.ciliumEndpointsLabels(context.TODO(), winPod)
	r.Unlock()
	require.NoError(t, err)
	la := lbls.LabelArray()

	sel := policytypes.ToSelector[api.CIDRRule]
	byLabels := func(ls *slim_metav1.LabelSelector) api.CIDRRule {
		return api.CIDRRule{CIDRGroupSelector: api.NewESFromK8sLabelSelector(labels.LabelSourceCIDRGroupKeyPrefix, ls)}
	}

	require.True(t, sel(api.CIDRRule{CIDRGroupRef: "win"}).Matches(la))
	require.False(t, sel(api.CIDRRule{CIDRGroupRef: "other"}).Matches(la))
	require.True(t, sel(byLabels(&slim_metav1.LabelSelector{MatchLabels: map[string]string{teamLabel: "x"}})).Matches(la))
	require.False(t, sel(byLabels(&slim_metav1.LabelSelector{MatchLabels: map[string]string{teamLabel: "y"}})).Matches(la))
	require.True(t, sel(byLabels(&slim_metav1.LabelSelector{MatchExpressions: []slim_metav1.LabelSelectorRequirement{
		{Key: teamLabel, Operator: slim_metav1.LabelSelectorOpExists},
	}})).Matches(la))
}

func TestCIDRGroupLabelsOnlyForWindowsWithFlag(t *testing.T) {
	g := cidrGroup("all", nil, "10.244.0.0/16")
	refKey := api.LabelForCIDRGroupRef("all").Key

	r := newCIDRGroupTestReconciler(t, Config{WindowsCIDRLabels: true})
	r.cidrGroupCache.upsert(g)
	_, linuxPod := podOnNode("linux-pod", "linux-node", "10.244.0.5")
	_, winPod := podOnNode("win-pod", "win-node", "10.244.1.57")
	r.Lock()
	linuxLbls, err := r.ciliumEndpointsLabels(context.TODO(), linuxPod)
	require.NoError(t, err)
	winLbls, err := r.ciliumEndpointsLabels(context.TODO(), winPod)
	require.NoError(t, err)
	r.Unlock()
	require.NotContains(t, linuxLbls, refKey, "linux pods keep upstream semantics")
	require.Contains(t, winLbls, refKey)

	off := newCIDRGroupTestReconciler(t, Config{})
	off.cidrGroupCache.upsert(g)
	off.Lock()
	offLbls, err := off.ciliumEndpointsLabels(context.TODO(), winPod)
	off.Unlock()
	require.NoError(t, err)
	require.NotContains(t, offLbls, refKey, "flag off adds no cidrgroup labels")
}

func TestCIDRGroupChangeRelabelsAffectedPods(t *testing.T) {
	r := newCIDRGroupTestReconciler(t, Config{WindowsOnlyEndpoints: true, WindowsCIDRLabels: true})
	refKey := api.LabelForCIDRGroupRef("g").Key
	ctx := context.TODO()

	inKey, inPod := podOnNode("win-in", "win-node", "10.244.1.57")
	outKey, outPod := podOnNode("win-out", "win-node", "10.244.1.99")
	require.NoError(t, r.ReconcilePod(ctx, inKey, inPod))
	require.NoError(t, r.ReconcilePod(ctx, outKey, outPod))
	outBefore, _ := r.store.GetPod(outKey)

	identity := func() int64 {
		p, ok := r.store.GetPod(inKey)
		require.True(t, ok)
		return p.identityID
	}
	idNoGroup := identity()

	// Group created: only the pod inside it is relabelled and gets a new identity.
	r.cidrGroupCache.upsert(cidrGroup("g", nil, "10.244.1.57/32"))
	require.NoError(t, r.reconcilePodsForCIDRGroup(ctx, "g"))
	in, _ := r.store.GetPod(inKey)
	require.Contains(t, in.lbls, refKey)
	idInGroup := identity()
	require.NotEqual(t, idNoGroup, idInGroup)
	outAfter, _ := r.store.GetPod(outKey)
	require.Equal(t, outBefore.identityID, outAfter.identityID)
	require.NotContains(t, outAfter.lbls, refKey)

	// Group edited to exclude the pod: it is found by its stored label and loses it.
	r.cidrGroupCache.upsert(cidrGroup("g", nil, "10.244.9.0/24"))
	require.NoError(t, r.reconcilePodsForCIDRGroup(ctx, "g"))
	in, _ = r.store.GetPod(inKey)
	require.NotContains(t, in.lbls, refKey)

	// Re-added, then deleted.
	r.cidrGroupCache.upsert(cidrGroup("g", nil, "10.244.1.0/24"))
	require.NoError(t, r.reconcilePodsForCIDRGroup(ctx, "g"))
	in, _ = r.store.GetPod(inKey)
	require.Contains(t, in.lbls, refKey)
	r.cidrGroupCache.delete("g")
	require.NoError(t, r.reconcilePodsForCIDRGroup(ctx, "g"))
	in, _ = r.store.GetPod(inKey)
	require.NotContains(t, in.lbls, refKey)
	require.Contains(t, in.lbls, "10.244.1.57/32", "cidr label stays")
}

func TestCIDRGroupLabelDoesNotReplacePodLabel(t *testing.T) {
	r := newCIDRGroupTestReconciler(t, Config{WindowsCIDRLabels: true})
	// The test pod has the k8s label k1=v1; a group with the same label key must not replace it.
	r.cidrGroupCache.upsert(cidrGroup("g", map[string]string{"k1": "other"}, "10.244.1.0/24"))
	_, winPod := podOnNode("win-pod", "win-node", "10.244.1.57")
	r.Lock()
	lbls, err := r.ciliumEndpointsLabels(context.TODO(), winPod)
	r.Unlock()
	require.NoError(t, err)
	require.Equal(t, labels.NewLabel("k1", "v1", labels.LabelSourceK8s), lbls["k1"])
	require.Contains(t, lbls, "k1"+labels.CIDRGroupEncodedSep+"other")
	require.Contains(t, lbls, api.LabelForCIDRGroupRef("g").Key)
}
