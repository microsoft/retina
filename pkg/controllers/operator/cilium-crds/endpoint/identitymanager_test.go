package endpointcontroller

import (
	"context"
	"strconv"
	"testing"

	ciliumutil "github.com/microsoft/retina/pkg/utils/testutil/cilium"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/cilium/cilium/pkg/labels"
	"github.com/cilium/cilium/pkg/option"
)

func TestGetIdentities(t *testing.T) {
	l := logrus.New()
	m := ciliumutil.NewMockVersionedClient(l, nil)

	// make sure to use CRD mode (this is referenced in InitIdentityAllocator)
	option.Config.IdentityAllocationMode = option.IdentityAllocationModeCRD
	im, err := NewIdentityManager(l, m)
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
	idObj, err := m.CiliumV2().CiliumIdentities().Get(context.TODO(), strconv.FormatInt(id, 10), metav1.GetOptions{})
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
	idObj, err = m.CiliumV2().CiliumIdentities().Get(context.TODO(), strconv.FormatInt(id3, 10), metav1.GetOptions{})
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
	l := logrus.New()
	m := ciliumutil.NewMockVersionedClient(l, nil)
	// make sure to use CRD mode (this is referenced in InitIdentityAllocator)
	option.Config.IdentityAllocationMode = option.IdentityAllocationModeCRD
	im, err := NewIdentityManager(l, m)
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
	idObj, err := m.CiliumV2().CiliumIdentities().Get(context.TODO(), strconv.FormatInt(id, 10), metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, strconv.FormatInt(id, 10), idObj.Name)
	idLabels := map[string]string{
		"k1":                          "v1",
		"io.kubernetes.pod.namespace": "x",
	}
	require.Equal(t, idLabels, idObj.Labels)
}
