// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
//go:build integration
// +build integration

package annotations

import (
	"testing"

	"github.com/microsoft/retina/pkg/log"
	common "github.com/microsoft/retina/test/integration/common"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	annotation_test_prefix = "test-forward-annotation-metrics-"
)

var (
	namespace string
	l         *log.ZapLogger
	ictx      *common.IntegrationContext
	k         *common.KubeManager
)

var expectedAnnotationTestMetrics = common.NewModelLocalCtxForwardMetrics(
	"egress",
	"",
	annotation_test_prefix,
	"client",
	"",
	"",
)

func TestAnnotation(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Annotations integration tests")
}

var _ = BeforeSuite(func() {
	ictx = common.Init()
	k = common.NewKubeManager(ictx.Logger(), ictx.Clientset)
	ictx.Logger().Info("Before Suite setup complete")
})

var _ = AfterSuite(func() {
	ictx.DumpAgentLogs()
	k.CleanupNamespaces()
	ictx.Logger().Info("After Suite cleanup complete")
})

var _ = Describe("Annotation Workflow Integration Tests", func() {
	var pre uint64
	var ac *AnnotationContext

	BeforeEach(func() {
		if !ictx.ValidateTestMode(common.LocalCtxMode) {
			Skip("Skipping annotation workflow test due to test mode")
		}
		l = ictx.Logger().Named("annotation-workflow")
		ac = InitAnnotationContext(ictx, k, expectedAnnotationTestMetrics.WithLabels(common.Count), annotation_test_prefix)
		pre = ac.GetTotalMetrics()
	})

	Context("Without annotations", func() {
		It("should validate that metrics are not emitted", func() {
			ac.CheckMetricsWithoutAnnotations(pre)
		})
	})

	Context("With both pod and namespace annotations", func() {
		It("should validate metric emissions", func() {
			ac.CheckMetricsWithAnnotations(pre)
		})

		It("should ensure independence between pod and namespace annotations", func() {
			ac.CheckAnnotationIndependence(pre)
		})
	})

	Context("With specific pod annotation", func() {
		It("should validate that metrics are confined to its node", func() {
			ac.CheckPodMetricsOnNode()
		})
	})
})
