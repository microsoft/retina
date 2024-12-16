package kubernetes

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

// PortForwarder can manage a port forwarding session.
type PortForwarder struct {
	clientset *kubernetes.Clientset
	transport http.RoundTripper
	upgrader  spdy.Upgrader
	logger    logger

	opts PortForwardingOpts

	stopChan    chan struct{}
	errChan     chan error
	address     string
	lazyAddress sync.Once
}

type PortForwardingOpts struct {
	Namespace     string
	LabelSelector string
	PodName       string
	LocalPort     int
	DestPort      int
}

// NewPortForwarder creates a PortForwarder.
func NewPortForwarder(restConfig *rest.Config, logger logger, opts PortForwardingOpts) (*PortForwarder, error) {
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("could not create clientset: %w", err)
	}

	transport, upgrader, err := spdy.RoundTripperFor(restConfig)
	if err != nil {
		return nil, fmt.Errorf("could not create spdy roundtripper: %w", err)
	}

	return &PortForwarder{
		clientset: clientset,
		transport: transport,
		upgrader:  upgrader,
		logger:    logger,
		opts:      opts,
		stopChan:  make(chan struct{}, 1),
	}, nil
}

// todo: can be made more flexible to allow a service to be specified

// Forward attempts to initiate port forwarding a pod and port using the configured namespace and labels.
// An error is returned if a port forwarding session could not be started. If no error is returned, the
// Address method can be used to communicate with the pod, and the Stop and KeepAlive methods can be used
// to manage the lifetime of the port forwarding session.

func (p *PortForwarder) Forward(ctx context.Context) error {
	var podName string
	if p.opts.PodName == "" {
		pods, err := p.clientset.CoreV1().Pods(p.opts.Namespace).List(ctx, metav1.ListOptions{LabelSelector: p.opts.LabelSelector, FieldSelector: "status.phase=Running"})
		if err != nil {
			return fmt.Errorf("could not list pods in %q with label %q: %w", p.opts.Namespace, p.opts.LabelSelector, err)
		}

		if len(pods.Items) < 1 {
			return fmt.Errorf("no pods found in %q with label %q", p.opts.Namespace, p.opts.LabelSelector) //nolint:goerr113 //no specific handling expected
		}

		randomIndex := rand.Intn(len(pods.Items)) //nolint:gosec //this is going to be revised in the future anyways, avoid random pods
		podName = pods.Items[randomIndex].Name
	} else {
		podName = p.opts.PodName
	}

	pods, err := p.clientset.CoreV1().Pods(p.opts.Namespace).List(ctx, metav1.ListOptions{LabelSelector: p.opts.LabelSelector, FieldSelector: "status.phase=Running"})
	if err != nil {
		return fmt.Errorf("could not list pods in %q with label %q: %w", p.opts.Namespace, p.opts.LabelSelector, err)
	}

	if len(pods.Items) < 1 {
		return fmt.Errorf("no pods found in %q with label %q", p.opts.Namespace, p.opts.LabelSelector) //nolint:goerr113 //no specific handling expected
	}

	portForwardURL := p.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(p.opts.Namespace).
		Name(podName).
		SubResource("portforward").URL()

	readyChan := make(chan struct{}, 1)
	dialer := spdy.NewDialer(p.upgrader, &http.Client{Transport: p.transport}, http.MethodPost, portForwardURL)
	ports := []string{fmt.Sprintf("%d:%d", p.opts.LocalPort, p.opts.DestPort)}
	pf, err := portforward.New(dialer, ports, p.stopChan, readyChan, io.Discard, io.Discard)
	if err != nil {
		return fmt.Errorf("could not create portforwarder: %w", err)
	}

	errChan := make(chan error, 1)
	go func() {
		// ForwardPorts is a blocking function thus it has to be invoked in a goroutine to allow callers to do
		// other things, but it can return 2 kinds of errors: initial dial errors that will be caught in the select
		// block below (Ready should not fire in these cases) and later errors if the connection is dropped.
		// this is why we propagate the error channel to PortForwardStreamHandle: to allow callers to handle
		// cases of eventual errors.
		errChan <- pf.ForwardPorts()
	}()

	var portForwardPort int
	select {
	case <-ctx.Done():
		return fmt.Errorf("portforward cancelled: %w", ctx.Err())
	case err := <-errChan:
		return fmt.Errorf("portforward failed: %w", err)
	case <-pf.Ready:
		prts, err := pf.GetPorts()
		if err != nil {
			return fmt.Errorf("get portforward port: %w", err)
		}

		if len(prts) < 1 {
			return errors.New("no ports forwarded")
		}

		portForwardPort = int(prts[0].Local)
	}

	// once successful, any subsequent port forwarding sessions from keep alive would yield the same address.
	// since the address could be read at the same time as the session is renewed, it's appropriate to initialize
	// lazily.
	p.lazyAddress.Do(func() {
		p.address = fmt.Sprintf("http://localhost:%d", portForwardPort)
	})

	p.errChan = errChan

	return nil
}

// Address returns an address for communicating with a port-forwarded pod.
func (p *PortForwarder) Address() string {
	return p.address
}

// Stop terminates a port forwarding session.
func (p *PortForwarder) Stop() {
	select {
	case p.stopChan <- struct{}{}:
	default:
	}
}

// KeepAlive can be used to restart the port forwarding session in the background.
func (p *PortForwarder) KeepAlive(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			p.logger.Logf("port forwarder: keep alive cancelled: %v", ctx.Err())
			return
		case <-p.stopChan:
			p.logger.Logf("port forwarder: keep alive stopped via stop channel")
			return
		case pfErr := <-p.errChan:
			// as of client-go v0.26.1, if the connection is successful at first but then fails,
			// an error is logged but only a nil error is sent to this channel. this will be fixed
			// in v0.27.x, which at the time of writing has not been released.
			//
			// see https://github.com/kubernetes/client-go/commit/d0842249d3b92ea67c446fe273f84fe74ebaed9f
			// for the relevant change.
			p.logger.Logf("port forwarder: received error signal: %v. restarting session", pfErr)
			p.Stop()
			if err := p.Forward(ctx); err != nil {
				p.logger.Logf("port forwarder: could not restart session: %v. retrying", err)

				select {
				case <-ctx.Done():
					p.logger.Logf("port forwarder: keep alive cancelled: %v", ctx.Err())
					return
				case <-time.After(time.Second): // todo: make configurable?
					continue
				}
			}
		}
	}
}
