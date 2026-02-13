package endpointcontroller

import (
	"context"
	"log/slog"
	"strconv"
	"testing"

	ciliumclient "github.com/cilium/cilium/pkg/k8s/client/testutils"

	"github.com/stretchr/testify/require"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/cilium/cilium/pkg/labels"
	"github.com/cilium/cilium/pkg/option"
)

func TestGetIdentities(t *testing.T) {
	// Skip due to client-go v0.35.0 bookmark event requirement incompatibility with Cilium's fake clientset
	// The fake clientset doesn't send bookmark events required by the new reflector implementation.
	// See: https://github.com/kubernetes/client-go/issues/1385
	// TODO: Fix by either upgrading Cilium's fake clientset or implementing a custom mock
	t.Skip("Skipping due to client-go v0.35.0 bookmark event requirement incompatibility with Cilium's fake clientset")

	ctx := context.Background()
	l := slog.Default()
	// Use Cilium's fake clientset which has proper watch support for the identity allocator
	fakeClientSet, _ := ciliumclient.NewFakeClientset(l)

	// make sure to use CRD mode (this is referenced in InitIdentityAllocator)
	option.Config.IdentityAllocationMode = option.IdentityAllocationModeCRD
	im, err := NewIdentityManager(ctx, l, fakeClientSet.CiliumFakeClientset)
	require.NoError(t, err)

	lbls := labels.Labels{
		"k1": labels.Label{
			Key:    "k1",
			Value:  "v1",
			Source: "k8s",
		},
		"io.kubernetes.pod.namespace": labels.Label{
			Key:    "io.kubernetes.pod.namespace",
			Value:  "x",
			Source: "k8s",
		},
	}

	id, err := im.GetIdentityAndIncrementReference(context.TODO(), lbls)

	require.NoError(t, err)
	require.Len(t, im.labelIdentities, 1)
	require.Greater(t, int(id), 0)

	// identity should be in API Server
	idObj, err := fakeClientSet.CiliumFakeClientset.CiliumV2().CiliumIdentities().Get(
		context.TODO(), strconv.FormatInt(id, 10), metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, strconv.FormatInt(id, 10), idObj.Name)
	idLabels := map[string]string{
		"k1":                          "v1",
		"io.kubernetes.pod.namespace": "x",
	}
	require.Equal(t, idLabels, idObj.Labels)

	// same labels should return the same identity
	id2, err := im.GetIdentityAndIncrementReference(context.TODO(), lbls)

	require.NoError(t, err)
	require.Equal(t, id, id2)
	require.Len(t, im.labelIdentities, 1)

	// new labels should return a new identity
	newLbls := labels.Labels{
		"k1": labels.Label{
			Key:    "k1",
			Value:  "v1",
			Source: "k8s",
		},
		"k2": labels.Label{
			Key:    "k2",
			Value:  "v2",
			Source: "k8s",
		},
		"io.kubernetes.pod.namespace": labels.Label{
			Key:    "io.kubernetes.pod.namespace",
			Value:  "x",
			Source: "k8s",
		},
	}

	id3, err := im.GetIdentityAndIncrementReference(context.TODO(), newLbls)

	require.NoError(t, err)
	require.NotEqual(t, id, id3)
	require.Len(t, im.labelIdentities, 2)
	require.Greater(t, int(id), 0)

	// identity should be in API Server
	idObj, err = fakeClientSet.CiliumFakeClientset.CiliumV2().CiliumIdentities().Get(
		context.TODO(), strconv.FormatInt(id3, 10), metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, strconv.FormatInt(id3, 10), idObj.Name)
	idLabels = map[string]string{
		"k1":                          "v1",
		"k2":                          "v2",
		"io.kubernetes.pod.namespace": "x",
	}
	require.Equal(t, idLabels, idObj.Labels)
}

func TestDecrementReference(t *testing.T) {
	// Skip due to client-go v0.35.0 bookmark event requirement incompatibility with Cilium's fake clientset
	// The fake clientset doesn't send bookmark events required by the new reflector implementation.
	// See: https://github.com/kubernetes/client-go/issues/1385
	// TODO: Fix by either upgrading Cilium's fake clientset or implementing a custom mock
	t.Skip("Skipping due to client-go v0.35.0 bookmark event requirement incompatibility with Cilium's fake clientset")

	ctx := context.Background()
	l := slog.Default()
	// Use Cilium's fake clientset which has proper watch support for the identity allocator
	fakeClientSet, _ := ciliumclient.NewFakeClientset(l)
	// make sure to use CRD mode (this is referenced in InitIdentityAllocator)
	option.Config.IdentityAllocationMode = option.IdentityAllocationModeCRD
	im, err := NewIdentityManager(ctx, l, fakeClientSet.CiliumFakeClientset)
	require.NoError(t, err)

	lbls := labels.Labels{
		"k1": labels.Label{
			Key:    "k1",
			Value:  "v1",
			Source: "k8s",
		},
		"io.kubernetes.pod.namespace": labels.Label{
			Key:    "io.kubernetes.pod.namespace",
			Value:  "x",
			Source: "k8s",
		},
	}

	id, err := im.GetIdentityAndIncrementReference(context.TODO(), lbls)
	require.NoError(t, err)
	_, err = im.GetIdentityAndIncrementReference(context.TODO(), lbls)
	require.NoError(t, err)

	// still a reference. identity should still exist
	im.DecrementReference(context.TODO(), lbls)
	require.Len(t, im.labelIdentities, 1)

	// no more references. identity should be deleted
	im.DecrementReference(context.TODO(), lbls)
	require.Empty(t, im.labelIdentities)

	// IdentityManager's allocator should not delete the identity (identitygc cell does garbage collection)
	idObj, err := fakeClientSet.CiliumFakeClientset.CiliumV2().CiliumIdentities().Get(
		context.TODO(), strconv.FormatInt(id, 10), metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, strconv.FormatInt(id, 10), idObj.Name)
	idLabels := map[string]string{
		"k1":                          "v1",
		"io.kubernetes.pod.namespace": "x",
	}
	require.Equal(t, idLabels, idObj.Labels)
}
