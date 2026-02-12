// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package retinaendpoint

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilrand "k8s.io/apimachinery/pkg/util/rand"

	retinav1alpha1 "github.com/microsoft/retina/crd/api/v1alpha1"
	retinaCommon "github.com/microsoft/retina/pkg/common"
)

func retinaEndpoitCmpComparer() cmp.Option {
	return cmp.Comparer(func(x, y *retinaCommon.RetinaEndpoint) bool {
		if (x != nil && y == nil) || (x == nil && y != nil) {
			return false
		}
		if x == nil && y == nil {
			return true
		}
		if !cmp.Equal(x.BaseObject, y.BaseObject, cmpopts.IgnoreTypes(sync.RWMutex{}), cmp.AllowUnexported(retinaCommon.BaseObject{})) {
			return false
		}
		if !cmp.Equal(x.Containers(), y.Containers()) {
			return false
		}
		if !cmp.Equal(x.OwnerRefs(), y.OwnerRefs()) {
			return false
		}
		if !cmp.Equal(x.Labels(), y.Labels()) {
			return false
		}
		if !cmp.Equal(x.Zone(), y.Zone()) {
			return false
		}
		return true
	})
}

var _ = Describe("Test Retina Capture Controller", func() {
	const (
		timeout  = time.Second * 10
		interval = time.Second * 1
	)
	var (
		node          *corev1.Node
		testNamespace string
		captureName   string
		captureRef    types.NamespacedName
		ns            corev1.Namespace
	)
	BeforeEach(func() {
		By("Creating a node with label 'kubernetes.io/role: agent'")
		node = &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node1",
				Labels: map[string]string{
					"kubernetes.io/role": "agent",
				},
			},
		}
		Expect(k8sClient.Create(context.Background(), node)).Should(Succeed())
	})

	AfterEach(func() {
		By("Deleting the node")
		Expect(k8sClient.Delete(ctx, node)).Should(Succeed())
	})

	Context("RetinaEndpoint is created successfully", func() {
		BeforeEach(func() {
			testNamespace = fmt.Sprintf("test-capture-%s", utilrand.String(5))
			By("Creating test namespace")
			ns = corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: testNamespace,
				},
			}
			Expect(k8sClient.Create(ctx, &ns)).Should(Succeed())

			captureName = "test-capture"
			captureRef = types.NamespacedName{Name: captureName, Namespace: testNamespace}
		})

		AfterEach(func() {
			By("Deleting test namespace")
			Expect(k8sClient.Delete(ctx, &ns)).Should(Succeed())
		})

		It("Should create capture successfully", func() {
			By("Add a RetinaNode to the cache")
			retinaNode := retinaCommon.NewRetinaNode("test-node", net.ParseIP("10.10.10.10"), "zone-1")
			Expect(retinaEndpointReconciler.cache.UpdateRetinaNode(retinaNode)).Should(Succeed())

			By("Creating a new RetinaEndpoint")
			ctx := context.Background()
			retinaEndpoint := &retinav1alpha1.RetinaEndpoint{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-capture",
					Namespace: testNamespace,
				},
				Spec: retinav1alpha1.RetinaEndpointSpec{
					NodeIP: "10.10.10.10",
					PodIP:  "10.0.0.1",
					PodIPs: []string{"10.0.0.1"},
					Containers: []retinav1alpha1.RetinaEndpointStatusContainers{
						{
							Name: "testcontainer",
							ID:   "docker://1234567890",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, retinaEndpoint)).Should(Succeed())

			createdRetinaEndpoint := &retinav1alpha1.RetinaEndpoint{}
			By("Checking if the RetinaEndpoint was successfully created")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, captureRef, createdRetinaEndpoint)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Checking the common RetinaEndpoint is created in Cache")
			expectedRetinaEndpointCommon := retinaCommon.RetinaEndpointCommonFromAPI(createdRetinaEndpoint, "zone-1")
			Eventually(func() string {
				retinaEndpointCommon := retinaEndpointReconciler.cache.GetPodByIP(createdRetinaEndpoint.Spec.PodIP)
				return cmp.Diff(retinaEndpointCommon, expectedRetinaEndpointCommon, retinaEndpoitCmpComparer())
			}, timeout, interval).Should(BeEmpty())

			By("Updating RetinaEndpoint")
			createdRetinaEndpoint.Spec.PodIP = "10.0.0.3"
			createdRetinaEndpoint.Spec.PodIPs = []string{"10.0.0.3"}
			Expect(k8sClient.Update(ctx, createdRetinaEndpoint)).Should(Succeed())

			By("Checking the common RetinaEndpoint is updated in Cache")
			expectedRetinaEndpointCommon = retinaCommon.RetinaEndpointCommonFromAPI(createdRetinaEndpoint, "zone-1")
			Eventually(func() string {
				retinaEndpointCommon := retinaEndpointReconciler.cache.GetPodByIP(retinaEndpoint.Spec.PodIP)
				return cmp.Diff(retinaEndpointCommon, expectedRetinaEndpointCommon, retinaEndpoitCmpComparer())
			}, timeout, interval).Should(BeEmpty())

			By("Deleting RetinaEndpoint")
			Expect(k8sClient.Delete(ctx, createdRetinaEndpoint)).Should(Succeed())

			By("Checking the common RetinaEndpoint is not in Cache")
			expectedRetinaEndpointCommon = retinaCommon.RetinaEndpointCommonFromAPI(createdRetinaEndpoint, "zone-1")
			Eventually(func() bool {
				retinaEndpointCommon := retinaEndpointReconciler.cache.GetPodByIP(retinaEndpoint.Spec.PodIP)
				return retinaEndpointCommon == nil
			}, timeout, interval).Should(BeTrue())
		})
	})
})
