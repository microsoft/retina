// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
//go:build integration
// +build integration

package capture

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	retinav1alpha1 "github.com/microsoft/retina/crd/api/v1alpha1"
	captureConstants "github.com/microsoft/retina/pkg/capture/constants"
	captureUtils "github.com/microsoft/retina/pkg/capture/utils"
)

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

	Context("Capture is created successfully", func() {
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

		It("Should create capture successfully when using Blob SAS URL", func() {
			By("Creating a Capture secret")
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: testNamespace,
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					captureConstants.CaptureOutputLocationBlobUploadSecretKey: []byte("blob sas url"),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).Should(Succeed())

			By("Creating a new Capture")
			hostPath := "/mnt/azure"
			ctx := context.Background()
			capture := &retinav1alpha1.Capture{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-capture",
					Namespace: testNamespace,
				},
				Spec: retinav1alpha1.CaptureSpec{
					CaptureConfiguration: retinav1alpha1.CaptureConfiguration{
						CaptureTarget: retinav1alpha1.CaptureTarget{
							NodeSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"kubernetes.io/role": "agent",
								},
							},
						},
						CaptureOption: retinav1alpha1.CaptureOption{
							Duration:       time.Minute * 1,
							MaxCaptureSize: 100,
						},
					},
					OutputConfiguration: retinav1alpha1.OutputConfiguration{
						HostPath:   &hostPath,
						BlobUpload: &secret.Name,
					},
				},
			}
			Expect(k8sClient.Create(ctx, capture)).Should(Succeed())

			createdCapture := &retinav1alpha1.Capture{}

			By("Checking if the Capture was successfully created")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, captureRef, createdCapture); err != nil {
					return false
				}
				return controllerutil.ContainsFinalizer(createdCapture, captureFinalizer)
			}, timeout, interval).Should(BeTrue())

			By("Checking the Capture is inProgress")
			Eventually(func() bool {
				Expect(k8sClient.Get(ctx, captureRef, createdCapture)).Should(Succeed())
				for _, condition := range createdCapture.Status.Conditions {
					if condition.Type == string(retinav1alpha1.CaptureComplete) && condition.Status == metav1.ConditionFalse {
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue())
			Expect(createdCapture.Status.Active).Should(Equal(int32(1)))
			Expect(createdCapture.Status.Failed).Should(Equal(int32(0)))
			Expect(createdCapture.Status.Succeeded).Should(Equal(int32(0)))

			By("Updating job status to completed")
			jobList := &batchv1.JobList{}
			Expect(k8sClient.List(ctx, jobList, client.InNamespace(testNamespace), client.MatchingLabels(captureUtils.GetJobLabelsFromCaptureName(capture.Name)))).Should(Succeed())
			Expect(len(jobList.Items) > 0).Should(BeTrue())
			for _, job := range jobList.Items {
				job.Status.Conditions = []batchv1.JobCondition{
					{
						Type:   batchv1.JobComplete,
						Status: corev1.ConditionTrue,
					},
				}
				Expect(k8sClient.Status().Update(ctx, &job)).Should(Succeed())
			}

			By("Waiting for capture's status to complete")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, captureRef, createdCapture); err != nil {
					return false
				}
				for _, condition := range createdCapture.Status.Conditions {
					if condition.Type == string(retinav1alpha1.CaptureComplete) && condition.Status == metav1.ConditionTrue {
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue())
			Expect(createdCapture.Status.Active).Should(Equal(int32(0)))
			Expect(createdCapture.Status.Failed).Should(Equal(int32(0)))
			Expect(createdCapture.Status.Succeeded).Should(Equal(int32(1)))
			Expect(createdCapture.Status.CompletionTime).ShouldNot(BeNil())

			By("Deleting the Capture")
			Expect(k8sClient.Delete(ctx, createdCapture)).Should(Succeed())

			By("Checking if jobs created has been deleted")
			Eventually(func() bool {
				Expect(k8sClient.List(ctx, jobList, client.InNamespace(testNamespace), client.MatchingLabels(captureUtils.GetJobLabelsFromCaptureName(capture.Name)))).Should(Succeed())
				return len(jobList.Items) == 0
			}, timeout, interval).Should(BeTrue())

			By("Checking if the Capture was successfully deleted")
			Eventually(func() bool {
				return apierrors.IsNotFound(k8sClient.Get(ctx, captureRef, createdCapture))
			}, timeout, interval).Should(BeTrue())
		})
	})
	Context("Capture is created with error", func() {
		var createdCapture *retinav1alpha1.Capture

		BeforeEach(func() {
			createdCapture = &retinav1alpha1.Capture{}

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
			By("Deleting the Capture")
			Expect(k8sClient.Delete(ctx, createdCapture)).Should(Succeed())

			jobList := &batchv1.JobList{}
			By("Checking if jobs created has been deleted")
			Eventually(func() bool {
				Expect(k8sClient.List(ctx, jobList, client.InNamespace(testNamespace), client.MatchingLabels(captureUtils.GetJobLabelsFromCaptureName(captureName)))).Should(Succeed())
				return len(jobList.Items) == 0
			}, timeout, interval).Should(BeTrue())

			By("Checking if the Capture was successfully deleted")
			Eventually(func() bool {
				return apierrors.IsNotFound(k8sClient.Get(ctx, captureRef, createdCapture))
			}, timeout, interval).Should(BeTrue())

			By("Deleting test namespace")
			Expect(k8sClient.Delete(ctx, &ns)).Should(Succeed())
		})
		It("Should create capture with error when running job failed", func() {
			By("creating a new Capture")
			hostPath := "/mnt/azure"
			ctx := context.Background()
			capture := &retinav1alpha1.Capture{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-capture",
					Namespace: testNamespace,
				},
				Spec: retinav1alpha1.CaptureSpec{
					CaptureConfiguration: retinav1alpha1.CaptureConfiguration{
						CaptureTarget: retinav1alpha1.CaptureTarget{
							NodeSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"kubernetes.io/role": "agent",
								},
							},
						},
						CaptureOption: retinav1alpha1.CaptureOption{
							Duration:       time.Minute * 1,
							MaxCaptureSize: 100,
						},
					},
					OutputConfiguration: retinav1alpha1.OutputConfiguration{
						HostPath: &hostPath,
					},
				},
			}
			Expect(k8sClient.Create(ctx, capture)).Should(Succeed())

			By("Checking if the Capture was successfully created")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, captureRef, createdCapture); err != nil {
					return false
				}
				return controllerutil.ContainsFinalizer(createdCapture, captureFinalizer)
			}, timeout, interval).Should(BeTrue())

			By("Checking if the Capture was inProgress")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, captureRef, createdCapture); err != nil {
					return false
				}
				completeCondition := metav1.Condition{}
				for _, condition := range createdCapture.Status.Conditions {
					if condition.Type == string(retinav1alpha1.CaptureComplete) {
						completeCondition = condition
						break
					}
				}
				return completeCondition.Status == metav1.ConditionFalse
			}, timeout, interval).Should(BeTrue())

			Expect(createdCapture.Status.Active).Should(Equal(int32(1)))
			Expect(createdCapture.Status.Failed).Should(Equal(int32(0)))
			Expect(createdCapture.Status.Succeeded).Should(Equal(int32(0)))

			By("Updating job status to failed")
			jobList := &batchv1.JobList{}
			Expect(k8sClient.List(ctx, jobList, client.InNamespace(testNamespace), client.MatchingLabels(captureUtils.GetJobLabelsFromCaptureName(capture.Name)))).Should(Succeed())
			Expect(len(jobList.Items) > 0).Should(BeTrue())
			for _, job := range jobList.Items {
				job.Status.Conditions = []batchv1.JobCondition{
					{
						Type:   batchv1.JobFailed,
						Status: corev1.ConditionTrue,
					},
				}
				Expect(k8sClient.Status().Update(ctx, &job)).Should(Succeed())
			}

			By("Waiting for capture's status to error")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, captureRef, createdCapture); err != nil {
					return false
				}
				for _, condition := range createdCapture.Status.Conditions {
					if condition.Type == string(retinav1alpha1.CaptureError) && condition.Status == metav1.ConditionTrue {
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue())

			Expect(createdCapture.Status.Active).Should(Equal(int32(0)))
			Expect(createdCapture.Status.Failed).Should(Equal(int32(1)))
			Expect(createdCapture.Status.Succeeded).Should(Equal(int32(0)))
		})

		It("Should create capture with error when secret is not found", func() {
			secretName := "test-secret"

			By("Creating a new Capture")
			hostPath := "/mnt/azure"
			ctx := context.Background()
			capture := &retinav1alpha1.Capture{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-capture",
					Namespace: testNamespace,
				},
				Spec: retinav1alpha1.CaptureSpec{
					CaptureConfiguration: retinav1alpha1.CaptureConfiguration{
						CaptureTarget: retinav1alpha1.CaptureTarget{
							NodeSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"kubernetes.io/role": "agent",
								},
							},
						},
						CaptureOption: retinav1alpha1.CaptureOption{
							Duration:       time.Minute * 1,
							MaxCaptureSize: 100,
						},
					},
					OutputConfiguration: retinav1alpha1.OutputConfiguration{
						HostPath:   &hostPath,
						BlobUpload: &secretName,
					},
				},
			}
			Expect(k8sClient.Create(ctx, capture)).Should(Succeed())

			By("Checking if the Capture was created with error")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, captureRef, createdCapture); err != nil {
					return false
				}
				for _, condition := range createdCapture.Status.Conditions {
					if condition.Type == string(retinav1alpha1.CaptureError) && condition.Status == metav1.ConditionTrue && condition.Reason == captureErrorReasonFindSecretFailed {
						return true
					}
				}
				return false
			}, timeout, interval).Should(BeTrue())
		})
	})
})
