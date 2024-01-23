// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
//go:build integration
// +build integration

package integration

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	config "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/log"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

const (
	jumpboxName   = "retina-jumpbox"
	jumpboxNs     = "default"
	timeout       = 60 * time.Second
	BasicMode     = "basic"
	LocalCtxMode  = "localctx"
	RemoteCtxMode = "remoteCtx"
)

var c *IntegrationContext

type IntegrationContext struct {
	l          *log.ZapLogger
	kubeconfig string
	config     *rest.Config

	jumpbox *corev1.Pod
	// Map agents to node name.
	nodeToAgent map[string]corev1.Pod

	Clientset         *kubernetes.Clientset
	CrdClient         *apiextensionsv1beta1.ApiextensionsV1beta1Client
	Nodes             *corev1.NodeList
	RetinaAgents      *corev1.PodList
	ControlPlaneNodes *corev1.NodeList
	ConfigMap         config.Config
	backoffStrategy   wait.Backoff
	LogPath           string
	enablePodLevel    bool
}

func Init() *IntegrationContext {
	if c != nil {
		return c
	}
	var err error

	c = &IntegrationContext{
		LogPath:        "/tmp/retina-integration-logs",
		enablePodLevel: false,
	}
	opts := log.GetDefaultLogOpts()
	opts.File = true
	opts.FileName = filepath.Join(c.LogPath, "retina.log")
	log.SetupZapLogger(opts)

	// Set backoff strategy.
	c.backoffStrategy = wait.Backoff{
		Duration: 10 * time.Millisecond,
		Jitter:   0,
		Factor:   2,
		Steps:    5,
	}

	c.l = log.Logger().Named("integration-test")
	// Delete log folder if exists.
	err = os.RemoveAll(c.LogPath)
	if err != nil {
		panic(err.Error())
	}
	err = os.Mkdir(c.LogPath, 0o755)
	if err != nil && !strings.Contains(err.Error(), "file exists") {
		panic(err.Error())
	}
	c.l.Info("Log path", zap.String("path", c.LogPath))
	c.kubeconfig = filepath.Join(homedir.HomeDir(), ".kube", "config")
	if c.kubeconfig == "" {
		panic("kubeconfig not set under $HOME/.kube/config")
	}
	c.l.Info("Using kubeconfig", zap.String("path", c.kubeconfig))
	c.config, err = clientcmd.BuildConfigFromFlags("", c.kubeconfig)
	if err != nil {
		panic(err.Error())
	}
	c.Clientset, err = kubernetes.NewForConfig(c.config)
	if err != nil {
		panic(err.Error())
	}
	c.CrdClient, err = apiextensionsv1beta1.NewForConfig(c.config)
	c.Nodes, err = c.Clientset.CoreV1().Nodes().List(context.TODO(), v1.ListOptions{
		LabelSelector: "!node-role.kubernetes.io/control-plane",
	})
	if err != nil {
		panic(err.Error())
	}
	if len(c.Nodes.Items) == 0 {
		panic("No agent nodes found")
	}
	c.ControlPlaneNodes, err = c.Clientset.CoreV1().Nodes().List(context.TODO(), v1.ListOptions{
		LabelSelector: "node-role.kubernetes.io/control-plane",
	})
	if err != nil {
		panic(err.Error())
	}
	c.RetinaAgents, err = c.Clientset.CoreV1().Pods("kube-system").List(context.TODO(), v1.ListOptions{
		LabelSelector: "app=retina",
	})
	if err != nil {
		panic(err.Error())
	}
	if len(c.RetinaAgents.Items) == 0 {
		panic("No retina agents found")
	}
	// Map agents to nodes.
	c.nodeToAgent = map[string]corev1.Pod{}
	for _, agent := range c.RetinaAgents.Items {
		for _, node := range c.Nodes.Items {
			// Split agent Node name by / to get the node name.
			agentNodeName := strings.Split(agent.Spec.NodeName, "/")[0]
			if agentNodeName == node.Name {
				c.nodeToAgent[node.Name] = agent
				c.l.Info("Mapped agent to node", zap.String("agent", agent.Name), zap.String("node", node.Name))
				break
			}
		}
	}
	c.jumpbox, err = c.Clientset.CoreV1().Pods("default").Create(context.TODO(), &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name: "retina-jumpbox",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "alpine",
					Image:   "mcr.microsoft.com/mirror/docker/library/alpine:3.16",
					Command: []string{"sleep", "infinity"},
				},
			},
			NodeSelector: map[string]string{
				"kubernetes.io/os":   "linux",
				"kubernetes.io/arch": "amd64",
			},
		},
	}, v1.CreateOptions{})
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		panic(err.Error())
	}
	// Wait for jumpbox to be ready.
	err = c.WaitForPodReady(jumpboxNs, jumpboxName)
	if err != nil {
		panic(err.Error())
	}

	enablePodLevel, err := strconv.ParseBool(os.Getenv("ENABLE_POD_LEVEL"))
	c.enablePodLevel = enablePodLevel
	c.l.Info("ENABLE_POD_LEVEL set", zap.Bool("ENABLE_POD_LEVEL", enablePodLevel))
	if err != nil {
		c.l.Warn("ENABLE_POD_LEVEL error parsing flag")
	}

	configMap, err := c.GetConfigMap("retina-config")
	c.ConfigMap = configMap
	if err != nil {
		c.l.Warn("Failed to parse config map", zap.Error(err))
	}

	c.l.Info("Remote context", zap.Bool("remoteContext", c.ConfigMap.RemoteContext))
	c.l.Info("Local context", zap.Bool("localContext", c.ConfigMap.EnableAnnotations))
	return c
}

func (ictx *IntegrationContext) IsEnablePodLevel() bool {
	return ictx.enablePodLevel
}

func (ictx *IntegrationContext) Logger() *log.ZapLogger {
	return ictx.l
}

func (ictx *IntegrationContext) FetchMetrics(node string) (string, error) {
	// Get agent IP.
	ictx.l.Info("Fetching metrics", zap.String("node", node))
	retinaAgentIp := ictx.nodeToAgent[node].Status.PodIP
	metricsUrl := fmt.Sprint("http://", retinaAgentIp, ":10093/metrics")
	cmd := exec.Command("kubectl", "exec", jumpboxName, "-n", jumpboxNs, "--", "wget", "-qO-", metricsUrl)
	out, err := cmd.Output()
	if err != nil {
		ictx.l.Error("Failed to fetch metrics", zap.Error(err), zap.String("output", string(out)))
		return "", err
	}
	return string(out), nil
}

func (ictx *IntegrationContext) WaitForPodReady(namespace string, name string) error {
	ctx, cancelFn := context.WithTimeout(context.Background(), timeout)
	defer cancelFn()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for pod %s to be ready", name)
		default:
			pod, err := ictx.Clientset.CoreV1().Pods(namespace).Get(context.Background(), name, v1.GetOptions{})
			if err != nil {
				// Maybe intermittent error, retry.
				continue
			}
			if pod.Status.Phase == corev1.PodRunning {
				return nil
			}
			time.Sleep(1 * time.Second)
		}
	}
}

func (ictx *IntegrationContext) GetNode(os string, arch string) (corev1.Node, error) {
	for _, node := range ictx.Nodes.Items {
		if strings.Contains(node.Labels["kubernetes.io/os"], os) && strings.Contains(node.Labels["kubernetes.io/arch"], arch) {
			return node, nil
		}
	}
	return corev1.Node{}, fmt.Errorf("node not found for os %s and arch %s", os, arch)
}

// Get two distinct nodes by OS
func (ictx *IntegrationContext) GetTwoDistinctNodes(os string) ([]corev1.Node, error) {
	nodes := []corev1.Node{}
	for _, node := range ictx.Nodes.Items {
		if strings.Contains(node.Labels["kubernetes.io/os"], os) {
			nodes = append(nodes, node)
		}
		if len(nodes) == 2 {
			break
		}
	}
	if len(nodes) < 2 {
		return nil, fmt.Errorf("less than two nodes found for os %s", os)
	}
	return nodes, nil
}

func (ictx *IntegrationContext) DumpAgentLogs() {
	agentLogPath := filepath.Join(ictx.LogPath, "agent")
	err := os.Mkdir(agentLogPath, 0o755)
	if err != nil && !strings.Contains(err.Error(), "file exists") {
		ictx.l.Error("Failed to create agent log folder", zap.Error(err))
		return
	}
	for _, agent := range ictx.RetinaAgents.Items {
		out := ictx.DumpPodLogs(&agent)
		agentLogFile := filepath.Join(agentLogPath, agent.Name+".log")
		err = os.WriteFile(agentLogFile, out, 0o644)
		if err != nil {
			ictx.l.Error("Failed to write agent log file", zap.Error(err))
			continue
		}
	}
}

func (ictx *IntegrationContext) DumpAllPodLogs(ns string) {
	pods, err := ictx.Clientset.CoreV1().Pods(ns).List(context.Background(), v1.ListOptions{})
	if err != nil {
		ictx.l.Error("Failed to list pods", zap.Error(err))
		return
	}
	podLogPath := filepath.Join(ictx.LogPath, ns)
	err = os.Mkdir(podLogPath, 0o755)
	if err != nil && !strings.Contains(err.Error(), "file exists") {
		ictx.l.Error("Failed to create pod log folder", zap.Error(err))
		return
	}
	for _, pod := range pods.Items {
		out := ictx.DumpPodLogs(&pod)
		podLogFile := filepath.Join(podLogPath, pod.Name+".log")
		err = os.WriteFile(podLogFile, out, 0o644)
		if err != nil {
			ictx.l.Error("Failed to write pod log file", zap.Error(err))
			continue
		}
	}
}

// Get a value of a config map key
func (ictx *IntegrationContext) GetConfigMap(cmName string) (config.Config, error) {
	ictx.l.Info("Getting config map", zap.String("name", cmName))
	var cm *corev1.ConfigMap
	operation := func() (bool, error) {
		var clientErr error
		cm, clientErr = ictx.Clientset.CoreV1().ConfigMaps("kube-system").Get(context.Background(), cmName, metav1.GetOptions{})
		if clientErr != nil {
			ictx.l.Warn("Failed to get configmap", zap.Error(clientErr))
			return false, clientErr
		}
		return true, nil
	}

	err := wait.ExponentialBackoff(c.backoffStrategy, operation)
	if err != nil {
		ictx.l.Error("Failed to get configmap after all retries", zap.Error(err))
	}

	cfgstring := strings.ReplaceAll(cm.Data["config.yaml"], "\\n", "\n")
	var data config.Config
	if err := yaml.Unmarshal([]byte(cfgstring), &data); err != nil {
		ictx.l.Error("Failed to unmarshal configmap", zap.Error(err), zap.Any("kcf", data))
		return data, err
	}
	return data, nil
}

func (ictx *IntegrationContext) DumpPodLogs(pod *corev1.Pod) []byte {
	execCmd := []string{"kubectl", "logs", pod.Name, "-n", pod.Namespace}
	cmd := exec.Command(execCmd[0], execCmd[1:]...)
	out, err := cmd.Output()
	if err != nil {
		ictx.l.Error("Failed to get pod logs", zap.Error(err))
		return []byte{}
	}
	return out
}

// ValidateTestMode checks if the test mode is enabled.
func (ictx *IntegrationContext) ValidateTestMode(testMode string) bool {
	if testMode == LocalCtxMode && !ictx.ConfigMap.EnableAnnotations {
		ictx.l.Info("Skipping test", zap.String("reason", "Local context not enabled"))
		return false
	}
	if testMode == RemoteCtxMode && !ictx.ConfigMap.RemoteContext {
		ictx.l.Info("Skipping test", zap.String("reason", "Remote context not enabled"))
		return false
	}
	return true
}

type MetricParser struct {
	// Body is the output from the metrics endpoint of Retina agent.
	Body string
}

// Parses metrics given labels. If parseAll is set to true,
// it will sum the values of all metrics that match the labels.
func (mp *MetricParser) Parse(metricLabels MetricWithLabels, parseAll bool) (uint64, error) {
	// Append metric name to labels.
	labels := metricLabels.Labels
	labels = append(labels, metricLabels.Metric)

	result := uint64(0)
	lines := strings.Split(mp.Body, "\n")
	for _, line := range lines {
		if mp.LineMatchesLabels(line, labels) {
			values := strings.Split(line, " ")
			flt, _, err := big.ParseFloat(values[1], 10, 0, big.ToNearestEven)
			if err != nil {
				return 0, err
			}
			val, _ := flt.Uint64()
			if !parseAll {
				return val, nil
			}
			result += val
		}
	}
	return result, nil
}

// Extracts lines that match all labels.
func (mp *MetricParser) ExtractMetricLines(metricLabels MetricWithLabels) []string {
	// Append metric name to labels.
	labels := metricLabels.Labels
	labels = append(labels, metricLabels.Metric)

	lines := strings.Split(mp.Body, "\n")
	result := []string{}
	for _, line := range lines {
		if mp.LineMatchesLabels(line, labels) {
			result = append(result, line)
		}
	}
	return result
}

// Checks if a line matches all labels.
func (mp *MetricParser) LineMatchesLabels(line string, labels []string) bool {
	if strings.HasPrefix(line, "#") {
		return false
	}
	for _, label := range labels {
		if label != "" && !strings.Contains(line, label) {
			return false
		}
	}
	return true
}
