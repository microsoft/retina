// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
//go:build integration
// +build integration

package forward

import (
	"strconv"
	"strings"
	"testing"
	"time"

	kcommon "github.com/microsoft/retina/pkg/common"
	"github.com/microsoft/retina/pkg/log"
	common "github.com/microsoft/retina/test/integration/common"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

const (
	namespace_prefix = "test-forward-metrics-"
)

var (
	namespace string
	l         *log.ZapLogger
	ictx      *common.IntegrationContext
	k         *common.KubeManager
)

var (
	expectedBasicForwardMetricIngress    = common.NewModelBasicForwardMetrics("ingress")
	expectedBasicForwardMetricEgress     = common.NewModelBasicForwardMetrics("egress")
	expectedLocalCtxForwardMetricIngress = common.NewModelLocalCtxForwardMetrics(
		"ingress",
		"",
		namespace_prefix,
		"server",
		"",
		"",
	)
	expectedLocalCtxForwardMetricEgress = common.NewModelLocalCtxForwardMetrics(
		"egress",
		"",
		namespace_prefix,
		"client",
		"",
		"",
	)

	expectedTcpFlagsMetrics = common.NewModelTcpFlagsMetrics(
		"SYN",
		"",
		namespace_prefix,
		"client",
		"",
		"",
	)
)

func TestForward(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Forward metrics integration tests")
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

var _ = Describe("Forward metrics integration tests", func() {
	BeforeEach(func() {
		// Create unique namespace. This is to make sure namespace deletion is not
		// interfering with other tests.
		namespace = namespace_prefix + strconv.FormatInt(time.Now().Unix(), 10)
		l = ictx.Logger().Named("forward-metrics")
	})

	AfterEach(func() {
		go ictx.DumpAllPodLogs(namespace)
	})

	DescribeTable("Packet count should spike",
		testLinuxPacketForward,
		getEntries()...,
	)
})

func getEntries() []TableEntry {
	tableEntries := []TableEntry{
		Entry("[BASIC_METRICS] packet forward linux",
			common.BasicMode,
			expectedBasicForwardMetricIngress.WithLabels(common.Count), expectedBasicForwardMetricIngress.WithLabels(common.Bytes),
			expectedBasicForwardMetricEgress.WithLabels(common.Count), expectedBasicForwardMetricEgress.WithLabels(common.Bytes)),
		Entry("[ADVANCED_METRICS_LOCAL_CTX] packet forward count linux with local context",
			common.LocalCtxMode,
			expectedLocalCtxForwardMetricIngress.WithLabels(common.Count), expectedLocalCtxForwardMetricIngress.WithLabels(common.Bytes),
			expectedLocalCtxForwardMetricEgress.WithLabels(common.Count), expectedLocalCtxForwardMetricEgress.WithLabels(common.Bytes)),
	}
	return tableEntries
}

func testLinuxPacketForward(testMode string, ingressCountLabels, ingressByteLabels, egressCountLabels, egressByteLabels common.MetricWithLabels) {
	// check valid advanced metrics scenario
	if !ictx.ValidateTestMode(testMode) {
		return
	}

	// check if advanced metrics is enabled
	isAdvancedMode := testMode != common.BasicMode

	// Get two linux nodes.
	nodes := []corev1.Node{}
	metrics := map[string]map[string][][2]uint64{}
	for _, node := range ictx.Nodes.Items {
		if strings.Contains(node.Labels["kubernetes.io/os"], "linux") {
			nodes = append(nodes, node)
			metrics[node.Name] = map[string][][2]uint64{}
			metrics[node.Name]["ingress"] = [][2]uint64{}
			metrics[node.Name]["egress"] = [][2]uint64{}
		}
		if len(nodes) == 2 {
			break
		}
	}
	if len(nodes) < 2 {
		l.Warn("Less than two linux nodes, skipping test")
		return
	}
	l.Info("Nodes chosen", zap.String("node1", nodes[0].Name), zap.String("node2", nodes[1].Name))

	f := func(checkTcpFlags, isAnnotated bool) {
		for _, node := range nodes {
			body, err := ictx.FetchMetrics(node.Name)
			Expect(err).To(BeNil())
			mp := &common.MetricParser{
				Body: body,
			}
			// check which node and modify the podname label accordingly
			if isAdvancedMode {
				ingressMetrics := expectedLocalCtxForwardMetricIngress
				egressMetrics := expectedLocalCtxForwardMetricEgress
				if node.Name == nodes[0].Name {
					expectedLocalCtxForwardMetricIngress.BaseObj.PodName = "server"
					expectedLocalCtxForwardMetricEgress.BaseObj.PodName = "server"
					expectedTcpFlagsMetrics.BaseObj.PodName = "server"

					ingressMetrics = expectedLocalCtxForwardMetricIngress
					egressMetrics = expectedLocalCtxForwardMetricEgress
				} else {
					expectedLocalCtxForwardMetricIngress.BaseObj.PodName = "client"
					expectedLocalCtxForwardMetricEgress.BaseObj.PodName = "client"
					expectedTcpFlagsMetrics.BaseObj.PodName = "client"

					ingressMetrics = expectedLocalCtxForwardMetricIngress
					egressMetrics = expectedLocalCtxForwardMetricEgress
				}
				// Obtain labels
				ingressCountLabels = ingressMetrics.WithLabels(common.Count)
				ingressByteLabels = ingressMetrics.WithLabels(common.Bytes)
				egressCountLabels = egressMetrics.WithLabels(common.Count)
				egressByteLabels = egressMetrics.WithLabels(common.Bytes)
			}

			// Parse ingress and egress metrics for count and bytes.
			for _, direction := range []string{"ingress", "egress"} {
				countLabels := ingressCountLabels
				bytesLabels := ingressByteLabels
				if direction == "egress" {
					countLabels = egressCountLabels
					bytesLabels = egressByteLabels
				}

				countVal, err := mp.Parse(countLabels, isAdvancedMode)
				Expect(err).To(BeNil())
				l.Info("Forward packet count / "+direction, zap.Any("labels", countLabels), zap.Uint64("count", countVal))

				bytesVal, err := mp.Parse(bytesLabels, isAdvancedMode)
				Expect(err).To(BeNil())
				l.Info("Forward packet bytes / "+direction, zap.Any("labels", bytesLabels), zap.Uint64("bytes", bytesVal))

				metrics[node.Name][direction] = append(metrics[node.Name][direction], [2]uint64{countVal, bytesVal})
			}

			// check for tcpflags metrics
			if isAdvancedMode && checkTcpFlags {
				lines := mp.ExtractMetricLines(expectedTcpFlagsMetrics.WithLabels())
				Expect(lines).NotTo(BeEmpty(), "tcpflags metrics are empty")
			}

		}
	}

	oneNsTwoPods := common.NewModelNamespace(namespace,
		common.NewModelPod("server", common.NewModelNode(nodes[0])).WithContainer(80, common.HTTP).WithContainer(81, common.TCP).WithContainer(82, common.UDP),
		common.NewModelPod("client", common.NewModelNode(nodes[1])).WithContainer(80, common.TCP))

	err := k.InitializeClusterFromModel(oneNsTwoPods)
	Expect(err).To(BeNil())

	// let networking stabilize after creating Pods
	time.Sleep(30 * time.Second)

	// Collect current state of metrics.
	f(false, false)

	// Annotate namespace in local context
	if testMode == common.LocalCtxMode {
		annotation := map[string]string{
			kcommon.RetinaPodAnnotation: kcommon.RetinaPodAnnotationValue,
		}
		err = k.AnnotateNamespace(namespace, annotation)
		Expect(err).To(BeNil())
	}

	seconds := 30
	time.Sleep(time.Duration(seconds) * time.Second)

	// Collect metrics again. This will be the baseline.
	f(false, true)

	k.ProbeRepeatedlyKind(namespace, "client", namespace, "server", common.HTTP, 80, seconds)

	// Collect metrics again.
	f(true, true)

	// Compare metrics.
	for _, node := range nodes {
		for _, dir := range []string{"ingress", "egress"} {
			baselineCount := metrics[node.Name][dir][1][0] - metrics[node.Name][dir][0][0]
			spikeCount := metrics[node.Name][dir][2][0] - metrics[node.Name][dir][1][0]
			l.Info("Metrics count", zap.String("node", node.Name), zap.String("dir", dir), zap.Uint64("baseline", baselineCount), zap.Uint64("spike", spikeCount))
			Expect(spikeCount).Should(BeNumerically(">", baselineCount))

			// For node level, the probes do not significantly increase the bytes sent or received.
			// Hence, we are not checking for difference between baseline and spike.
			baselineBytes := metrics[node.Name][dir][1][1]
			spikeBytes := metrics[node.Name][dir][2][1]
			if testMode != common.BasicMode {
				baselineBytes = metrics[node.Name][dir][1][1] - metrics[node.Name][dir][0][1]
				spikeBytes = metrics[node.Name][dir][2][1] - metrics[node.Name][dir][1][1]
			}
			l.Info("Metrics bytes", zap.String("node", node.Name), zap.String("dir", dir), zap.Uint64("baseline", baselineBytes), zap.Uint64("spike", spikeBytes))
			Expect(spikeBytes).Should(BeNumerically(">", baselineBytes))
		}
	}
}
