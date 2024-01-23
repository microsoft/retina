// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
//go:build integration
// +build integration

package dropreason

import (
	"strconv"
	"testing"
	"time"

	kcommon "github.com/microsoft/retina/pkg/common"
	"github.com/microsoft/retina/pkg/log"
	common "github.com/microsoft/retina/test/integration/common"
	"go.uber.org/zap"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

const (
	namespace_prefix       = "test-drops-metrics-"
	annotation_test_prefix = "test-drops-annotation-metrics-"
)

var (
	namespace string
	l         *log.ZapLogger
	ictx      *common.IntegrationContext
	k         *common.KubeManager
)

var (
	expectedBasicDropMetrics = common.NewModelBasicDropReasonMetrics(
		"unknown",
		"IPTABLE_RULE_DROP",
	)
	expectedTcpFlagsMetrics = common.NewModelTcpFlagsMetrics(
		"SYN",
		"",
		namespace_prefix,
		"client",
		"",
		"",
	)
	expectedLocalContextDropReasonMetrics = common.NewModelLocalCtxDropReasonMetrics(
		"egress",
		"",
		namespace_prefix,
		"client",
		"IPTABLE_RULE_DROP",
		"",
		"",
	)
)

func TestDrops(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Drop metrics and annotations integration tests")
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

var _ = Describe("Drop metrics integration tests", func() {
	BeforeEach(func() {
		// Create unique namespace. This is to make sure namespace deletion is not
		// interfering with other tests.
		namespace = namespace_prefix + strconv.FormatInt(time.Now().Unix(), 10)
		l = ictx.Logger().Named("drop-metrics")
	})

	AfterEach(func() {
		go ictx.DumpAllPodLogs(namespace)
	})

	DescribeTable("dropped packet count should increase for nodes",
		testLinuxPacketDrop,
		getAdvancedMetricsEntries()...,
	)
})

func getAdvancedMetricsEntries() []TableEntry {
	tableEntries := []TableEntry{
		Entry(
			"[BASIC_METRICS] drop linux/amd64",
			common.BasicMode, common.LinuxAmd64.OS, common.LinuxAmd64.Arch,
			expectedBasicDropMetrics.WithLabels(common.Count), expectedBasicDropMetrics.WithLabels(common.Bytes)),
		Entry("[BASIC_METRICS] drop linux/arm64",
			common.BasicMode, common.LinuxArm64.OS, common.LinuxArm64.Arch,
			expectedBasicDropMetrics.WithLabels(common.Count), expectedBasicDropMetrics.WithLabels(common.Bytes)),
		Entry("[ADVANCED_METRICS_LOCAL_CTX] drop linux/amd64",
			common.LocalCtxMode, common.LinuxAmd64.OS, common.LinuxAmd64.Arch,
			expectedLocalContextDropReasonMetrics.WithLabels(common.Count), expectedLocalContextDropReasonMetrics.WithLabels(common.Bytes)),
		Entry("[ADVANCED_METRICS_LOCAL_CTX] drop linux/arm64", common.LocalCtxMode, common.LinuxArm64.OS, common.LinuxArm64.Arch,
			expectedLocalContextDropReasonMetrics.WithLabels(common.Count), expectedLocalContextDropReasonMetrics.WithLabels(common.Bytes)),
	}
	return tableEntries
}

func testLinuxPacketDrop(testMode, os, arch string, countLabels, bytesLabels common.MetricWithLabels) {
	l.Info("starting testLinuxPacketDrop", zap.Any("OS", os), zap.Any("Arch", arch))

	// Skip test if test mode is not enabled.
	if !ictx.ValidateTestMode(testMode) {
		return
	}

	// set isAdvancedMode to true for advanced metrics
	isAdvancedMode := testMode != common.BasicMode

	// Choose a linux/arch node.
	node, err := ictx.GetNode(os, arch)
	if err != nil {
		l.Warn("No node of given os-arch, skipping test", zap.Any("OS", os), zap.Any("Arch", arch))
		return
	}
	l.Info("Node chosen", zap.String("node", node.Name))
	// Use the node choosen to deploy server/client.
	n := common.NewModelNode(node)

	err = k.InitializeClusterFromModel(common.NewModelNamespace(namespace,
		common.NewModelPod("server", n).WithContainer(80, common.HTTP),
		common.NewModelPod("client", n).WithContainer(80, common.TCP),
	))
	Expect(err).To(BeNil())

	// Collect current state of metrics.
	mp := fetchMetricParser(node.Name)

	// Annotate namespace in local context
	if testMode == common.LocalCtxMode {
		// annotate namespace
		annotation := map[string]string{
			kcommon.RetinaPodAnnotation: kcommon.RetinaPodAnnotationValue,
		}

		// Annotate client pod
		err = k.AnnotatePod("client", namespace, annotation)
		Expect(err).To(BeNil())

		// Annotate server
		err = k.AnnotatePod("server", namespace, annotation)
		Expect(err).To(BeNil())

		// Annotate namespace
		err = k.AnnotateNamespace(namespace, annotation)
		Expect(err).To(BeNil())

		// collect metrics again
		mp = fetchMetricParser(node.Name)

	}
	metricOldCount := parseMetricsValue(countLabels, isAdvancedMode, mp)
	metricOldBytes := parseMetricsValue(bytesLabels, isAdvancedMode, mp)
	oldTcpFlagsCount := parseMetricsValue(expectedTcpFlagsMetrics.WithLabels(), isAdvancedMode, mp)

	// Apply Network Policy to drop traffic to nginx pod.
	err = k.ApplyNetworkDropPolicy(namespace, "deny-server-ingress", common.PodLabelKey, "server")
	Expect(err).To(BeNil())

	k.ProbeRepeatedlyKind(namespace, "client", namespace, "server", common.HTTP, 80, 20)

	// Collect metrics again.
	mp = fetchMetricParser(node.Name)
	metricNewCount := parseMetricsValue(countLabels, isAdvancedMode, mp)
	metricNewBytes := parseMetricsValue(bytesLabels, isAdvancedMode, mp)

	Expect(metricNewCount).Should(BeNumerically(">", metricOldCount))
	Expect(metricNewBytes).Should(BeNumerically(">", metricOldBytes))

	// Check tcpflags metrics for advanced metrics
	if isAdvancedMode {
		newTcpFlagsCount := parseMetricsValue(expectedTcpFlagsMetrics.WithLabels(), isAdvancedMode, mp)
		Expect(newTcpFlagsCount).Should(BeNumerically(">", oldTcpFlagsCount))
	}
}

// FetchMetricParser fetches the metrics from the node and returns the MetricParser object
func fetchMetricParser(nodeName string) *common.MetricParser {
	body, err := ictx.FetchMetrics(nodeName)
	Expect(err).To(BeNil())
	mp := &common.MetricParser{
		Body: body,
	}
	return mp
}

// Fetched the count for a given label
func parseMetricsValue(metricLabels common.MetricWithLabels, isAdvancedMode bool, mp *common.MetricParser) uint64 {
	val, err := mp.Parse(metricLabels, isAdvancedMode)
	Expect(err).To(BeNil())
	l.Info("drop packet", zap.Any("labels", metricLabels), zap.Uint64("value", val))
	return val
}
