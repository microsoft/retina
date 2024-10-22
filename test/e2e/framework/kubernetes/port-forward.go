// todo: matmerr, this is just going to remain broken until it can be validated with scenarios pr

package kubernetes

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	retry "github.com/microsoft/retina/test/retry"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	defaultTimeoutSeconds    = 300
	defaultRetryDelay        = 500 * time.Millisecond
	defaultRetryAttempts     = 60
	defaultHTTPClientTimeout = 2 * time.Second
)

var (
	ErrNoPodWithLabelFound = fmt.Errorf("no pod with label found with matching pod affinity")

	defaultRetrier = retry.Retrier{Attempts: defaultRetryAttempts, Delay: defaultRetryDelay, ExpBackoff: true}
)

type PortForward struct {
	Namespace             string
	LabelSelector         string
	LocalPort             string
	RemotePort            string
	Endpoint              string
	KubeConfigFilePath    string
	OptionalLabelAffinity string

	// local properties
	pf *PortForwarder
}

func (p *PortForward) Run() error {
	lport, _ := strconv.Atoi(p.LocalPort)
	rport, _ := strconv.Atoi(p.RemotePort)

	pctx := context.Background()
	portForwardCtx, cancel := context.WithTimeout(pctx, defaultTimeoutSeconds*time.Second)
	defer cancel()

	config, err := clientcmd.BuildConfigFromFlags("", p.KubeConfigFilePath)
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("could not create clientset: %w", err)
	}

	// if we have an optional label affinity, find a pod with that label, on the same node as a pod with the label selector
	targetPodName := ""
	if p.OptionalLabelAffinity != "" {
		// get all pods with label
		log.Printf("attempting to find pod with label \"%s\", on a node with a pod with label \"%s\"\n", p.LabelSelector, p.OptionalLabelAffinity)
		targetPodName, err = p.findPodsWithAffinity(pctx, clientset)
		if err != nil {
			return fmt.Errorf("could not find pod with affinity: %w", err)
		}
	}

	portForwardFn := func() error {
		// if we have a pod name (likely from affinity above), use it, otherwise use label selector
		opts := PortForwardingOpts{
			Namespace: p.Namespace,
			PodName:   targetPodName,
			LocalPort: lport,
			DestPort:  rport,
		}

		if targetPodName != "" {
			opts.PodName = targetPodName
		}

		log.Printf("attempting port forward to pod name \"%s\" with label \"%s\", in namespace \"%s\"...\n", targetPodName, p.LabelSelector, p.Namespace)

		p.pf, err = NewPortForwarder(config, logger{}, opts)
		if err != nil {
			return fmt.Errorf("could not create port forwarder: %w", err)
		}
		err = p.pf.Forward(pctx)
		if err != nil {
			return fmt.Errorf("could not start port forward: %w", err)
		}

		// verify port forward succeeded
		client := http.Client{
			Timeout: defaultHTTPClientTimeout,
		}
		resp, err := client.Get(p.pf.Address() + "/" + p.Endpoint) //nolint
		if err != nil {
			log.Printf("port forward validation HTTP request to %s failed: %v\n", p.pf.Address(), err)
			p.pf.Stop()
			return fmt.Errorf("port forward validation HTTP request to %s failed: %w", p.pf.Address(), err)
		}
		defer resp.Body.Close()

		log.Printf("port forward validation HTTP request to \"%s\" succeeded, response: %s\n", p.pf.Address(), resp.Status)

		return nil
	}

	if err = defaultRetrier.Do(portForwardCtx, portForwardFn); err != nil {
		return fmt.Errorf("could not start port forward within %ds: %w", defaultTimeoutSeconds, err)
	}
	log.Printf("successfully port forwarded to \"%s\"\n", p.pf.Address())
	return nil
}

func (p *PortForward) findPodsWithAffinity(ctx context.Context, clientset *kubernetes.Clientset) (string, error) {
	targetPodsAll, errAffinity := clientset.CoreV1().Pods(p.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: p.LabelSelector,
		FieldSelector: "status.phase=Running",
	})
	if errAffinity != nil {
		return "", fmt.Errorf("could not list pods in %q with label %q: %w", p.Namespace, p.LabelSelector, errAffinity)
	}

	// omit windows pods because we can't port-forward to them
	targetPodsLinux := make([]v1.Pod, 0)
	for i := range targetPodsAll.Items {
		if targetPodsAll.Items[i].Spec.NodeSelector["kubernetes.io/os"] != "windows" {
			targetPodsLinux = append(targetPodsLinux, targetPodsAll.Items[i])
		}
	}

	// get all pods with optional label affinity
	affinityPods, errAffinity := clientset.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
		LabelSelector: p.OptionalLabelAffinity,
		FieldSelector: "status.phase=Running",
	})
	if errAffinity != nil {
		return "", fmt.Errorf("could not list affinity pods across all namespaces with label %q: %w", p.OptionalLabelAffinity, errAffinity)
	}

	// keep track of where the affinity pods are scheduled
	affinityNodes := make(map[string]bool)
	for i := range affinityPods.Items {
		affinityNodes[affinityPods.Items[i].Spec.NodeName] = true
	}

	// if a pod is found on the same node as an affinity pod, use it
	for i := range targetPodsLinux {
		if affinityNodes[targetPodsLinux[i].Spec.NodeName] {
			// found a pod with the specified label, on a node with the optional label affinity
			return targetPodsLinux[i].Name, nil
		}
	}

	return "", fmt.Errorf("could not find a pod with label \"%s\", on a node that also has a pod with label \"%s\": %w", p.LabelSelector, p.OptionalLabelAffinity, ErrNoPodWithLabelFound)
}

func (p *PortForward) Prevalidate() error {
	return nil
}

func (p *PortForward) Stop() error {
	p.pf.Stop()
	return nil
}

type logger struct{}

func (l *logger) Logf(format string, args ...interface{}) {
	log.Printf(format, args...)
}
