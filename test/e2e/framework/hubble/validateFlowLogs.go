package hubble

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	k8s "github.com/microsoft/retina/test/e2e/framework/kubernetes"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/transport/spdy"
	"k8s.io/kubectl/pkg/scheme"
)

const (
	namespace             = "test-hubble"
	hubbleRelayNamespace  = "kube-system"
	hubbleRelayService    = "service/hubble-relay"
	hubbleRelayPort       = 4245
	serverServiceName     = "server-service"
	clientPodName         = "client"
	serverPodName         = "server"
	setupWaitTime         = 10 * time.Second
	hubbleObservationTime = 10 * time.Second
	curlTimeout           = "5"
)

// ValidateHubbleFlowLogs validates Hubble flow logs
type ValidateHubbleFlowLogs struct {
	KubeConfigFilePath string
}

func (v *ValidateHubbleFlowLogs) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	config, clientset, err := v.setupKubernetesClient()
	if err != nil {
		return err
	}

	// Create resources
	if err := v.createResources(ctx, clientset); err != nil {
		return err
	}
	defer v.cleanupResources(ctx, clientset)

	// Setup port forwarding
	stopChan, err := v.portForwardToHubbleRelay()
	if err != nil {
		return err
	}
	defer func() {
		log.Println("Stopping hubble-relay port-forward...")
		if stopChan != nil {
			close(stopChan)
		}
	}()

	time.Sleep(setupWaitTime)

	// Start observing Hubble flows
	hubbleOutput, err := v.observeHubbleFlows(ctx, clientset, config)
	if err != nil {
		return err
	}

	// Analyze the flow logs
	if !v.analyzeFlowLogs(hubbleOutput) {
		return errors.New("failed to validate Hubble flow logs")
	}
	log.Println("Hubble flow logs validated successfully!")

	return nil
}

func (v *ValidateHubbleFlowLogs) Prevalidate() error {
	log.Println("Prevalidating Hubble flow logs...")
	if v.KubeConfigFilePath == "" {
		return errors.New("KubeConfigFilePath is required")
	}
	// Verify that hubble is installed on the local machine
	cmd := exec.Command("hubble", "version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("hubble command not found: %w", err)
	}
	return nil
}

func (f *ValidateHubbleFlowLogs) Stop() error {
	return nil
}

func (c *ValidateHubbleFlowLogs) getNamespace(namespace string) *v1.Namespace {
	return &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}
}

func (c *ValidateHubbleFlowLogs) getServerService() *v1.Service {
	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "server-service",
			Namespace: namespace,
		},
		Spec: v1.ServiceSpec{
			Type: v1.ServiceTypeClusterIP,
			Ports: []v1.ServicePort{
				{
					Port:       80,
					TargetPort: intstr.FromInt(80),
				},
			},
			Selector: map[string]string{"app": "server"},
		},
	}
}

func (c *ValidateHubbleFlowLogs) getClientDeployment() *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "client",
			Namespace: namespace,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "client",
					Image: "curlimages/curl:latest",
					Command: []string{
						"sleep",
						"3600",
					},
				},
			},
		},
	}
}

func (c *ValidateHubbleFlowLogs) getServerDeployment() *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "server",
			Namespace: namespace,
			Labels: map[string]string{
				"app": "server",
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "server",
					Image: "nginx:latest",
					Ports: []v1.ContainerPort{
						{
							ContainerPort: 80,
						},
					},
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      "nginx-config",
							MountPath: "/etc/nginx/conf.d/default.conf",
							SubPath:   "default.conf",
						},
					},
				},
			},
			Volumes: []v1.Volume{
				{
					Name: "nginx-config",
					VolumeSource: v1.VolumeSource{
						ConfigMap: &v1.ConfigMapVolumeSource{
							LocalObjectReference: v1.LocalObjectReference{
								Name: "nginx-config",
							},
						},
					},
				},
			},
		},
	}
}

func (v *ValidateHubbleFlowLogs) setupKubernetesClient() (*rest.Config, *kubernetes.Clientset, error) {
	config, err := clientcmd.BuildConfigFromFlags("", v.KubeConfigFilePath)
	if err != nil {
		return nil, nil, fmt.Errorf("error building kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating Kubernetes client: %w", err)
	}

	return config, clientset, nil
}

func (v *ValidateHubbleFlowLogs) createResources(ctx context.Context, clientset *kubernetes.Clientset) error {
	log.Println("Creating resources...")
	resources := []runtime.Object{
		v.getNamespace(namespace),
		v.getServerService(),
		v.getClientDeployment(),
		v.getServerDeployment(),
	}

	for _, resource := range resources {
		if err := k8s.CreateResource(ctx, resource, clientset); err != nil {
			return fmt.Errorf("error creating resource: %w", err)
		}
	}
	log.Println("Waiting for server pod to be ready...")
	waitCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := v.waitForPod(waitCtx, clientset, serverPodName, namespace); err != nil {
		return fmt.Errorf("error waiting for server pod: %w", err)
	}
	log.Println("Server pod is ready.")
	log.Println("Waiting for client pod to be ready...")
	if err := v.waitForPod(ctx, clientset, clientPodName, namespace); err != nil {
		return fmt.Errorf("error waiting for client pod: %w", err)
	}
	log.Println("Client pod is ready.")

	return nil
}

func (v *ValidateHubbleFlowLogs) waitForPod(ctx context.Context, clientset *kubernetes.Clientset, podName, ns string) error {
	log.Printf("Waiting for pod %s in namespace %s to be ready...\n", podName, ns)
	// Poll until the pod is ready
	for {
		pod, err := clientset.CoreV1().Pods(ns).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get pod %s: %w", podName, err)
		}

		// Check if pod is ready
		for _, condition := range pod.Status.Conditions {
			if condition.Type == v1.PodReady && condition.Status == v1.ConditionTrue {
				return nil
			}
		}

		time.Sleep(2 * time.Second)
	}
}

func (v *ValidateHubbleFlowLogs) cleanupResources(ctx context.Context, clientset *kubernetes.Clientset) {
	log.Println("Deleting resources...")
	resources := []runtime.Object{
		v.getNamespace(namespace),
		v.getServerService(),
		v.getClientDeployment(),
		v.getServerDeployment(),
	}

	for _, resource := range resources {
		if err := k8s.DeleteResource(ctx, resource, clientset); err != nil {
			log.Printf("error deleting resource: %v", err)
		}
	}
}

func (v *ValidateHubbleFlowLogs) portForwardToHubbleRelay() (chan struct{}, error) {
	log.Println("Setting up port-forward to hubble-relay...")
	stopChan, err := v.setupPortForward(hubbleRelayNamespace, hubbleRelayService, hubbleRelayPort, hubbleRelayPort)
	if err != nil {
		return nil, fmt.Errorf("failed to set up port-forward to hubble-relay: %w", err)
	}
	return stopChan, nil
}

func (v *ValidateHubbleFlowLogs) observeHubbleFlows(ctx context.Context, clientset *kubernetes.Clientset, config *rest.Config) (string, error) {
	log.Println("Starting Hubble observation...")
	var hubbleOutputBuffer bytes.Buffer

	hubbleCmd := exec.Command("hubble", "observe",
		"--namespace", namespace,
		"--protocol", "TCP",
		"-f")

	hubbleCmd.Stdout = &hubbleOutputBuffer
	hubbleCmd.Stderr = os.Stderr

	if err := hubbleCmd.Start(); err != nil {
		return "", fmt.Errorf("error starting hubble command: %w", err)
	}

	// Ensure hubble command is stopped when we're done
	defer func() {
		if hubbleCmd.Process != nil {
			hubbleCmd.Process.Kill()
		}
	}()

	// Allow a moment for hubble to start capturing
	time.Sleep(hubbleObservationTime)

	err := v.execInPod(
		ctx,
		clientset,
		config,
		namespace,
		clientPodName,
		[]string{"curl", "-s", "-m", curlTimeout, "http://" + serverServiceName},
		os.Stderr,
		false,
	)
	if err != nil {
		return "", fmt.Errorf("failed to execute curl command in client pod: %w", err)
	}

	time.Sleep(hubbleObservationTime)

	hubbleOutput := hubbleOutputBuffer.String()

	// Check if hubble output is empty
	if len(hubbleOutput) == 0 {
		return "", errors.New("no hubble flow logs captured")
	}

	log.Printf("Captured Hubble Flows: \n%s", hubbleOutput)
	return hubbleOutput, nil
}

func (c *ValidateHubbleFlowLogs) execInPod(
	ctx context.Context,
	clientset *kubernetes.Clientset,
	config *rest.Config,
	namespace, podName string,
	command []string,
	stderr io.Writer,
	tty bool,
) error {
	log.Printf("Executing command %v on pod %s in namespace %s", command, podName, namespace)

	req := clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec")

	option := &v1.PodExecOptions{
		Command: command,
		Stderr:  true,
		TTY:     tty,
	}

	req.VersionedParams(
		option,
		scheme.ParameterCodec,
	)

	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("error creating SPDY executor: %w", err)
	}

	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stderr: stderr,
		Tty:    tty,
	})

	if err != nil {
		return errors.Wrapf(err, "error executing command %v on pod %s", command, podName)
	}
	return nil
}

// Set up port forwarding to a pod or service
// This function sets up port forwarding to a specified pod or service in a Kubernetes cluster.
// It takes the namespace, resource reference (pod/service), local port, and remote port as arguments.
// Returns a channel to stop the port forwarding and an error if any occurs.
func (c *ValidateHubbleFlowLogs) setupPortForward(namespace, resourceRef string, localPort, remotePort int) (chan struct{}, error) {
	// Load kubeconfig
	config, err := c.getKubeConfig()
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	// Get target pod name from resource reference
	targetPod, err := c.getTargetPodName(clientset, namespace, resourceRef)
	if err != nil {
		return nil, err
	}

	// Set up port forwarding
	return c.createPortForwarder(config, namespace, targetPod, localPort, remotePort)
}

func (c *ValidateHubbleFlowLogs) getKubeConfig() (*rest.Config, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		kubeconfig := clientcmd.RecommendedHomeFile
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
		}
	}
	return config, nil
}

func (c *ValidateHubbleFlowLogs) getTargetPodName(
	clientset *kubernetes.Clientset,
	namespace,
	resourceRef string,
) (string, error) {
	parts := strings.SplitN(resourceRef, "/", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid resource reference: %s", resourceRef)
	}
	resourceType, resourceName := parts[0], parts[1]

	switch resourceType {
	case "pod":
		// Just verify pod exists
		_, err := clientset.CoreV1().Pods(namespace).Get(context.Background(), resourceName, metav1.GetOptions{})
		if err != nil {
			return "", fmt.Errorf("failed to find pod %s: %w", resourceName, err)
		}
		return resourceName, nil

	case "service":
		// Resolve to pod behind service
		endpoints, err := clientset.CoreV1().Endpoints(namespace).Get(context.Background(), resourceName, metav1.GetOptions{})
		if err != nil {
			return "", fmt.Errorf("failed to get endpoints for service %s: %w", resourceName, err)
		}
		if len(endpoints.Subsets) == 0 || len(endpoints.Subsets[0].Addresses) == 0 {
			return "", fmt.Errorf("no pods found behind service %s", resourceName)
		}
		return endpoints.Subsets[0].Addresses[0].TargetRef.Name, nil

	default:
		return "", fmt.Errorf("unsupported resource type: %s", resourceType)
	}
}

func (c *ValidateHubbleFlowLogs) createPortForwarder(
	config *rest.Config,
	namespace,
	podName string,
	localPort,
	remotePort int,
) (chan struct{}, error) {
	transport, upgrader, err := spdy.RoundTripperFor(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create spdy round tripper: %w", err)
	}

	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", namespace, podName)
	hostIP := strings.TrimPrefix(config.Host, "https://")
	serverURL := url.URL{Scheme: "https", Path: path, Host: hostIP}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, &serverURL)
	ports := []string{fmt.Sprintf("%d:%d", localPort, remotePort)}

	stopChan := make(chan struct{}, 1)
	readyChan := make(chan struct{})

	pf, err := portforward.New(dialer, ports, stopChan, readyChan, os.Stdout, os.Stderr)
	if err != nil {
		return nil, fmt.Errorf("failed to create port forwarder: %w", err)
	}

	go func() {
		if err := pf.ForwardPorts(); err != nil {
			log.Printf("Port forwarding error: %v", err)
		}
	}()

	select {
	case <-readyChan:
		log.Printf("Port forward ready: localhost:%d -> %s:%d\n", localPort, podName, remotePort)
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("timed out waiting for port forward to be ready")
	}

	return stopChan, nil
}

// Analyze hubble flow logs to verify connectivity
func (c *ValidateHubbleFlowLogs) analyzeFlowLogs(hubbleOutput string) bool {
	log.Println("Analyzing Hubble flow logs...")

	type flowStep struct {
		name        string
		observed    bool
		pattern     *regexp.Regexp
		description string // Kept for logging/documentation
	}

	// Precompile the regex patterns for better performance
	steps := map[string]*flowStep{
		"clientInitiatedSYN": {
			name:        "Client initiated connection (SYN)",
			observed:    false,
			pattern:     regexp.MustCompile(`client.*?-> server.*?SYN:true`),
			description: "client -> server with SYN flag",
		},
		"serverRespondedSYNACK": {
			name:        "Server responded (SYN-ACK)",
			observed:    false,
			pattern:     regexp.MustCompile(`client.*?<- server.*?SYN:true.*?ACK:true`),
			description: "server -> client with SYN+ACK flags",
		},
		"clientSentACK": {
			name:        "Client acknowledged (ACK)",
			observed:    false,
			pattern:     regexp.MustCompile(`client.*?-> server.*?ACK:true`),
			description: "client -> server with ACK flag (no SYN)",
		},
		"clientSentData": {
			name:        "Client sent data",
			observed:    false,
			pattern:     regexp.MustCompile(`client.*?-> server.*?PSH:true`),
			description: "client -> server with PSH flag",
		},
		"serverRespondedData": {
			name:        "Server responded with data",
			observed:    false,
			pattern:     regexp.MustCompile(`client.*?<- server.*?PSH:true`),
			description: "server -> client with PSH flag",
		},
		"clientInitiatedClose": {
			name:        "Client initiated connection close",
			observed:    false,
			pattern:     regexp.MustCompile(`client.*?-> server.*?FIN:true`),
			description: "client -> server with FIN flag",
		},
	}

	// Additional check for pure ACK packets (without SYN)
	hasPureAck := false
	synAckPattern := regexp.MustCompile(`client.*?-> server.*?ACK:true`)
	synPattern := regexp.MustCompile(`SYN:true`)

	// Regex to detect incorrect flow direction
	incorrectFlowPattern := regexp.MustCompile(`-> client`)
	incorrectFlowDirection := false

	for line := range strings.SplitSeq(hubbleOutput, "\n") {
		if line == "" {
			continue
		}

		// Check for incorrect flow direction
		if incorrectFlowPattern.MatchString(line) {
			incorrectFlowDirection = true
			log.Printf("⚠️ Detected incorrect flow direction: %s\n", line)
		}

		// Special handling for pure ACK packets
		if synAckPattern.MatchString(line) && !synPattern.MatchString(line) {
			hasPureAck = true
		}

		// Check each flow step using regex patterns
		for key, step := range steps {
			if step.pattern.MatchString(line) {
				// For client ACK, only mark observed if we found a pure ACK
				if key == "clientSentACK" {
					steps[key].observed = hasPureAck
				} else {
					steps[key].observed = true
				}
			}
		}
	}

	// Check if all required flow steps were observed
	allStepsObserved := true
	log.Printf("Connection steps observed:\n")

	for _, step := range steps {
		allStepsObserved = allStepsObserved && step.observed
		log.Printf("	- %s: %v\n", step.name, step.observed)
	}
	log.Printf("	- All flows have correct direction: %v\n", !incorrectFlowDirection)

	allStepsObserved = allStepsObserved && !incorrectFlowDirection

	if allStepsObserved {
		log.Printf("✅	Connectivity test successful! Complete TCP handshake, data transfer and connection teardown observed.\n")
	} else {
		log.Printf("⚠️	Connectivity verification incomplete.\n")
	}

	return allStepsObserved
}
