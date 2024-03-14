// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
//go:build integration
// +build integration

package dns

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
	namespacePrefix = "test-dns-metrics-"
	dnsTestPod      = "dns-test-pod"
	domainName      = "kubernetes.default"
)

var (
	namespace string
	l         *log.ZapLogger
	ictx      *common.IntegrationContext
	k         *common.KubeManager
)

var (
	expectedDnsRequestMetrics = common.NewModelDnsCountMetrics(
		"",
		domainName,
		"A",
		"",
		"",
		"",
		namespacePrefix,
		dnsTestPod,
		"",
		"",
	)

	expectedDnsResponseMetrics = common.NewModelDnsCountMetrics(
		"",
		domainName,
		"A",
		"",
		"NOERROR",
		"",
		namespacePrefix,
		dnsTestPod,
		"",
		"",
	)
	// coreDnsResponseMetrics = common.NewModelDnsCountMetrics(

)

func TestDns(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "DNS integration tests")
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

var _ = Describe("DNS integration tests", func() {
	BeforeEach(func() {
		// Create unique namespace. This is to make sure namespace deletion is not
		// interfering with other tests.
		namespace = namespacePrefix + strconv.FormatInt(time.Now().Unix(), 10)
		l = ictx.Logger().Named("dns-metrics")
	})

	AfterEach(func() {
		go ictx.DumpAllPodLogs(namespace)
	})

	DescribeTable("dns count should increase for nodes",
		testDnsMetrics,
		getMetricsEntries()...,
	)
})

func getMetricsEntries() []TableEntry {
	tableEntries := []TableEntry{
		Entry("[ADVANCED_METRICS_LOCAL_CTX] dns count",
			common.LocalCtxMode,
			expectedDnsRequestMetrics.WithLabels(common.Request), expectedDnsResponseMetrics.WithLabels(common.Response)),
	}
	return tableEntries
}

// TestDnsMetrics tests the dns metrics by sending nslookup requests to kubernetes.default
// NsLookup send A requests and is expecting to receive NOERROR response from the dns server.
func testDnsMetrics(testMode string, requestLabels, responseLabels common.MetricWithLabels) {
	l.Info("starting testDnsMetrics")

	// Skip test if test mode is not enabled.
	if !ictx.ValidateTestMode(testMode) {
		return
	}

	// Choose a linux/arch node.
	node, err := ictx.GetNode("linux", "")
	if err != nil {
		l.Warn("No linux/amd64 node, skipping test")
		return
	}
	l.Info("Node chosen", zap.String("node", node.Name))
	// Use the node chosen to deploy server/client.
	n := common.NewModelNode(node)

	err = k.InitializeClusterFromModel(common.NewModelNamespace(namespace,
		common.NewModelPod(dnsTestPod, n).WithContainer(80, common.HTTP),
	))
	Expect(err).To(BeNil())

	// Annotate namespace in localctx mode.
	if testMode == common.LocalCtxMode {
		// annotate pod
		annotation := map[string]string{
			kcommon.RetinaPodAnnotation: kcommon.RetinaPodAnnotationValue,
		}

		err = k.AnnotateNamespace(namespace, annotation)
		Expect(err).To(BeNil())

	}
	// Collect current state of metrics.
	mp := fetchMetricParser(node.Name)

	metricOldRequest := parseMetricsValue(requestLabels, mp)
	metricOldResponse := parseMetricsValue(responseLabels, mp)

	// Send nslookup requests to bing.com
	err = k.PerformNslookup(namespace, dnsTestPod, domainName)
	Expect(err).To(BeNil())

	// Collect metrics again.
	mp = fetchMetricParser(node.Name)
	metricNewRequest := parseMetricsValue(requestLabels, mp)
	metricNewResponse := parseMetricsValue(responseLabels, mp)

	Expect(metricNewRequest).Should(BeNumerically(">", metricOldRequest))
	Expect(metricNewResponse).Should(BeNumerically(">", metricOldResponse))
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
func parseMetricsValue(metricLabels common.MetricWithLabels, mp *common.MetricParser) uint64 {
	val, err := mp.Parse(metricLabels, true)
	Expect(err).To(BeNil())
	l.Info("dns packet", zap.Any("labels", metricLabels), zap.Uint64("value", val))
	return val
}
