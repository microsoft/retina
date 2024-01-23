// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

//go:build integration
// +build integration

package annotations

import (
	"strconv"
	"strings"
	"time"

	kcommon "github.com/microsoft/retina/pkg/common"
	"github.com/microsoft/retina/pkg/log"
	common "github.com/microsoft/retina/test/integration/common"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
)

var a *AnnotationContext

type AnnotationContext struct {
	l           *log.ZapLogger
	Ictx        *common.IntegrationContext
	KubeManager *common.KubeManager
	Pods        []*common.ModelPod
	Namespace   *common.ModelNamespace
	Metrics     common.MetricWithLabels
	Nodes       []common.ModelNode
	annotation  map[string]string
}

// create init function to initialize the annotation context
func InitAnnotationContext(ictx *common.IntegrationContext, k *common.KubeManager, annotationMetrics common.MetricWithLabels, namespace_prefix string) *AnnotationContext {
	if a != nil {
		return a
	}

	a = &AnnotationContext{
		Ictx:        ictx,
		KubeManager: k,
		Metrics:     annotationMetrics,
		l:           log.Logger().Named("annotation-tests"),
	}

	// Choose two linux nodes
	nodes, err := ictx.GetTwoDistinctNodes("linux")
	if err != nil {
		panic(err.Error())
	}

	a.l.Info("Nodes chosen", zap.String("node1", nodes[0].Name), zap.String("node2", nodes[1].Name))

	clientPod := common.NewModelPod("client", common.NewModelNode(nodes[0])).WithContainer(80, common.HTTP)
	serverPod := common.NewModelPod("server", common.NewModelNode(nodes[1])).WithContainer(80, common.HTTP)

	a.Pods = []*common.ModelPod{clientPod, serverPod}
	a.Nodes = []common.ModelNode{common.NewModelNode(nodes[0]), common.NewModelNode(nodes[1])}

	// Create unique namespace. This is to make sure namespace deletion is not
	namespace := namespace_prefix + strconv.FormatInt(time.Now().Unix(), 10)

	twoPodsInSameNamespace := common.NewModelNamespace(namespace, clientPod, serverPod)
	a.Namespace = twoPodsInSameNamespace

	a.annotation = map[string]string{
		kcommon.RetinaPodAnnotation: kcommon.RetinaPodAnnotationValue,
	}

	a.l = log.Logger().Named("annotation-tests")

	err = k.InitializeClusterFromModel(twoPodsInSameNamespace)
	if err != nil {
		panic(err.Error())
	}

	return a
}

// Check that no metrics are emitted when no annotations are present.
func (ac *AnnotationContext) CheckMetricsWithoutAnnotations(pre uint64) {
	ac.performProbe(false)
	post := ac.GetTotalMetrics()

	Expect(pre).To(Equal(post), "No annotations were present, so pre and post annotation values should be equal")
	ac.l.Info("CheckMetricsWithoutAnnotations completed successfully")
}

// Check that metrics are emitted when annotations are present.
func (ac *AnnotationContext) CheckMetricsWithAnnotations(pre uint64) {
	ac.annotatePods()
	// TODO - uncomment this once we have a fix for namespace annotation removal issue https://github.com/microsoft/retina/issues/667
	// ac.annotateNamespace()
	ac.performProbe(false)
	post := ac.GetTotalMetrics()

	Expect(post).To(
		BeNumerically(">", pre),
		"annotation exists, so post annotation count should be greater than pre annotation count")
	ac.l.Info("CheckMetricsWithAnnotations completed successfully")
}

// Check that removing the annotation from a pod doesn't affect namespace metrics, and vice versa
func (ac *AnnotationContext) CheckAnnotationIndependence(pre uint64) {
	annotatedns := ac.KubeManager.GetAnnotatedNamespaces(ac.annotation)
	ac.l.Info("Annotated namespaces before removal", zap.Any("annotatedns", annotatedns))

	annotatedPods := ac.KubeManager.GetAnnotatedPods(ac.Namespace.BaseName, ac.annotation)
	ac.l.Info("Annotated pods before removal", zap.Any("annotatedPods", annotatedPods))

	ac.removeNamespaceAnnotations()
	ac.performProbe(false)
	post := ac.GetTotalMetrics()

	// post annotation count should be greater than pre annotation count, because pod annotation still exists
	Expect(post).To(
		BeNumerically(">", pre),
		"Pod annotation still exists, so post annotation count should be greater than pre annotation count")

	ac.removePodAnnotations()

	annotatedns = ac.KubeManager.GetAnnotatedNamespaces(ac.annotation)
	ac.l.Info("Annotated namespaces after removal", zap.Any("annotatedns", annotatedns))

	annotatedPods = ac.KubeManager.GetAnnotatedPods(ac.Namespace.BaseName, ac.annotation)
	ac.l.Info("Annotated pods after removal", zap.Any("annotatedPods", annotatedPods))

	pre = ac.GetTotalMetrics() // update pre count

	ac.performProbe(false)
	post = ac.GetTotalMetrics()

	// post annotation count should be equal to pre annotation count, because pod annotation was removed
	Expect(pre).To(Equal(post), "All annotations were removed, so pre and post annotation values should be equal")

	// Annotate both namespace and pods then remove pod annotation
	ac.annotateNamespace()
	ac.annotatePods()
	ac.removePodAnnotations()

	pre = ac.GetTotalMetrics() // update pre count
	ac.performProbe(false)
	post = ac.GetTotalMetrics()

	Expect(post).To(
		BeNumerically(">", pre),
		"Namespace annotation still exists, so post annotation count should be greater than pre annotation count")
	ac.l.Info("CheckAnnotationIndependence completed successfully")
}

// Check metrics of an annotated pod are only on the node it's running on.
func (ac *AnnotationContext) CheckPodMetricsOnNode() {
	if len(ac.Pods) != 2 || len(ac.Nodes) != 2 {
		panic("This test requires exactly 2 pods and 2 nodes")
	}

	ac.removePodAnnotations()
	ac.removeNamespaceAnnotations()

	for i, pod := range ac.Pods {
		// if metric is contains drop reason, reverse the network policy
		if strings.Contains(ac.Metrics.Metric, "drop") {
			ac.ReverseNetworkPolicy()
		}
		// Annotate the pod and get initial metrics.
		err := ac.KubeManager.AnnotatePod(pod.Name, ac.Namespace.BaseName, ac.annotation)
		Expect(err).To(BeNil())

		initialMetrics := ac.getNodeMetrics(ac.Nodes[i], pod.Name)

		// Perform a probe and get updated metrics.
		ac.performProbe(i == 1)
		updatedMetrics := ac.getNodeMetrics(ac.Nodes[i], pod.Name)

		Expect(updatedMetrics).To(
			BeNumerically(">", initialMetrics),
			"Metrics not emitted for annotated pod %s on node %s", pod.Name, ac.Nodes[i].HostName)

		// Ensure the other node metrics remain the same.
		otherNodeInitialMetrics := ac.getNodeMetrics(ac.Nodes[1-i], pod.Name)
		ac.performProbe(i == 1)
		otherNodeUpdatedMetrics := ac.getNodeMetrics(ac.Nodes[1-i], pod.Name)

		Expect(otherNodeInitialMetrics).To(
			Equal(otherNodeUpdatedMetrics),
			"Metrics emitted for non annotated pod %s on a different node %s", pod.Name, ac.Nodes[1-i].HostName)

		// Cleanup: remove the annotation from the current pod.
		err = ac.KubeManager.RemovePodAnnotations(pod.Name, ac.Namespace.BaseName, ac.annotation)
		Expect(err).To(BeNil())

	}
	ac.l.Info("CheckPodMetricsOnNode completed successfully")
}

// Fetch and parse metrics for a specific node.
func (ac *AnnotationContext) getNodeMetrics(node common.ModelNode, podName string) uint64 {
	updatedLabels := ac.updatePodNameLabel(podName)

	mp := ac.fetchMetricParser(node.HostName)
	return ac.parseMetricsValue(updatedLabels, true, mp)
}

// FetchMetricParser fetches the metrics from the node and returns the MetricParser object
func (ac *AnnotationContext) fetchMetricParser(nodeName string) *common.MetricParser {
	body, err := ac.Ictx.FetchMetrics(nodeName)
	Expect(err).To(BeNil())
	mp := &common.MetricParser{
		Body: body,
	}
	return mp
}

// Fetched the count for a given label
func (ac *AnnotationContext) parseMetricsValue(metricLabels common.MetricWithLabels, isAdvancedMode bool, mp *common.MetricParser) uint64 {
	val, err := mp.Parse(metricLabels, isAdvancedMode)
	Expect(err).To(BeNil())
	ac.l.Info("drop packet", zap.Any("labels", metricLabels), zap.Uint64("value", val))
	return val
}

// Common function to get the total metrics from all nodes
func (ac *AnnotationContext) GetTotalMetrics() uint64 {
	total := uint64(0)
	for _, node := range ac.Nodes {
		mp := ac.fetchMetricParser(node.HostName)
		total += ac.parseMetricsValue(ac.Metrics, true, mp)
	}
	return total
}

// Common function to perform a probe
// Perform a probe from client to server if reversed is false, else from server to client
func (ac *AnnotationContext) performProbe(reversed bool) {
	clientPod := ac.Pods[0]
	serverPod := ac.Pods[1]

	if reversed {
		clientPod, serverPod = serverPod, clientPod
	}

	ac.KubeManager.ProbeRepeatedlyKind(ac.Namespace.BaseName, clientPod.Name, ac.Namespace.BaseName, serverPod.Name, common.HTTP, 80, 10)
}

// Common function to annotate pods
func (ac *AnnotationContext) annotatePods() {
	for _, pod := range ac.Pods {
		err := ac.KubeManager.AnnotatePod(pod.Name, ac.Namespace.BaseName, ac.annotation)
		Expect(err).To(BeNil())
	}
}

// Common function to annotate namespace
func (ac *AnnotationContext) annotateNamespace() {
	err := ac.KubeManager.AnnotateNamespace(ac.Namespace.BaseName, ac.annotation)
	Expect(err).To(BeNil())
}

// Common function to remove pod annotations
func (ac *AnnotationContext) removePodAnnotations() {
	for _, pod := range ac.Pods {
		err := ac.KubeManager.RemovePodAnnotations(pod.Name, ac.Namespace.BaseName, ac.annotation)
		Expect(err).To(BeNil())
	}
}

// Common function to remove namespace annotations
func (ac *AnnotationContext) removeNamespaceAnnotations() {
	err := ac.KubeManager.RemoveNamespaceAnnotations(ac.Namespace.BaseName, ac.annotation)
	Expect(err).To(BeNil())
}

// Common function to update pod name label
func (ac *AnnotationContext) updatePodNameLabel(podName string) common.MetricWithLabels {
	metricWithLabels := ac.Metrics
	// Look for the "podname" label and modify its value
	for i := 0; i < len(metricWithLabels.Labels); i += 2 {
		if metricWithLabels.Labels[i] == "podname" && i+1 < len(metricWithLabels.Labels) {
			metricWithLabels.Labels[i+1] = podName
			break
		}
	}
	return metricWithLabels
}

// Common function to reverse network policy from server to client
func (ac *AnnotationContext) ReverseNetworkPolicy() {
	err := ac.KubeManager.RemoveNetworkPolicy(ac.Namespace.BaseName, "deny-server-ingress")
	Expect(err).To(BeNil())
	ac.KubeManager.ApplyNetworkDropPolicy(ac.Namespace.BaseName, "deny-client-ingress", common.PodLabelKey, "client")
	Expect(err).To(BeNil())
}
