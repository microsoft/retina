// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
//go:build integration
// +build integration

// this file provides the Namespace and Pod structs that emulate those in upstream NetPol e2e:
// https://github.com/kubernetes/kubernetes/blob/master/test/e2e/network/netpol/model.go

package integration

import (
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	PodLabelKey = "pod"

	agnHostImage = "mcr.microsoft.com/aks/e2e/k8s-agnhost:2.39"
)

type ModelNode struct {
	OS       string
	Arch     string
	HostName string
}

func NewModelNode(n v1.Node) ModelNode {
	return ModelNode{
		OS:       n.Labels["kubernetes.io/os"],
		Arch:     n.Labels["kubernetes.io/arch"],
		HostName: n.Labels["kubernetes.io/hostname"],
	}
}

var (
	LinuxAmd64 = ModelNode{
		OS:   "linux",
		Arch: "amd64",
	}

	LinuxArm64 = ModelNode{
		OS:   "linux",
		Arch: "arm64",
	}

	LinuxAnyArch = ModelNode{
		OS:   "linux",
		Arch: "",
	}

	// TODO windows node types
)

type ModelProtocol string

const (
	TCP  ModelProtocol = "TCP"
	UDP  ModelProtocol = "UDP"
	HTTP ModelProtocol = "HTTP"
)

func (p ModelProtocol) ToV1Proto() v1.Protocol {
	if p == UDP {
		return v1.ProtocolUDP
	}
	return v1.ProtocolTCP
}

// ModelNamespace is an abstract representation i.e. ignores kube implementation details
type ModelNamespace struct {
	BaseName string
	Pods     []*ModelPod
}

// NewModelNamespace creates a Namespace with the given Pods
func NewModelNamespace(name string, pods ...*ModelPod) *ModelNamespace {
	return &ModelNamespace{
		BaseName: name,
		Pods:     pods,
	}
}

// ModelPod is an abstract representation i.e. ignores kube implementation details
type ModelPod struct {
	Name       string
	Node       ModelNode
	Containers []*ModelContainer
}

// NewModelPod can be chained with WithContainer
func NewModelPod(name string, n ModelNode) *ModelPod {
	return &ModelPod{
		Name: name,
		Node: n,
	}
}

// WithContainer appends the specified container to the Pod and returns the same object
func (p *ModelPod) WithContainer(port int32, proto ModelProtocol) *ModelPod {
	p.Containers = append(p.Containers, &ModelContainer{
		Port:     port,
		Protocol: proto,
	})
	return p
}

// ContainerSpecs builds kubernetes container specs for the pod
func (p *ModelPod) ContainerSpecs() []v1.Container {
	var containers []v1.Container
	for _, cont := range p.Containers {
		containers = append(containers, cont.Spec())
	}
	return containers
}

// Labels returns the default labels that should be placed on a pod/deployment
// in order for it to be uniquely selectable by label selectors
func (p *ModelPod) Labels() map[string]string {
	return map[string]string{
		PodLabelKey: p.Name,
	}
}

// KubePod returns the kube pod (will add label selectors for windows if needed).
func (p *ModelPod) KubePod(namespace string) *v1.Pod {
	zero := int64(0)

	kPod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      p.Name,
			Labels:    p.Labels(),
			Namespace: namespace,
		},
		Spec: v1.PodSpec{
			TerminationGracePeriodSeconds: &zero,
			Containers:                    p.ContainerSpecs(),
		},
	}

	kPod.Spec.NodeSelector = map[string]string{
		"kubernetes.io/os": p.Node.OS,
	}

	if p.Node.Arch != "" {
		kPod.Spec.NodeSelector["kubernetes.io/arch"] = p.Node.Arch
	}
	if p.Node.HostName != "" {
		kPod.Spec.NodeSelector["kubernetes.io/hostname"] = p.Node.HostName
	}

	return kPod
}

// QualifiedServiceAddress returns the address that can be used to access the service
func (p *ModelPod) QualifiedServiceAddress(namespace string, dnsDomain string) string {
	return fmt.Sprintf("%s.%s.svc.%s", p.ServiceName(namespace), namespace, dnsDomain)
}

// ServiceName returns the unqualified service name
func (p *ModelPod) ServiceName(namespace string) string {
	return fmt.Sprintf("s-%s-%s", namespace, p.Name)
}

// Service returns a kube service spec
func (p *ModelPod) Service(namespace string) *v1.Service {
	service := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      p.ServiceName(namespace),
			Namespace: namespace,
		},
		Spec: v1.ServiceSpec{
			Selector: p.Labels(),
		},
	}

	for _, container := range p.Containers {
		service.Spec.Ports = append(service.Spec.Ports, v1.ServicePort{
			Name:     fmt.Sprintf("service-port-%s-%d", strings.ToLower(string(container.Protocol)), container.Port),
			Protocol: container.Protocol.ToV1Proto(),
			Port:     container.Port,
		})
	}
	return service
}

// ModelContainer is an abstract representation i.e. ignores kube implementation details
type ModelContainer struct {
	Port     int32
	Protocol ModelProtocol
}

// Name returns the container name
func (c *ModelContainer) Name() string {
	return fmt.Sprintf("cont-%d-%s", c.Port, strings.ToLower(string(c.Protocol)))
}

// PortName returns the container port name
func (c *ModelContainer) PortName() string {
	return fmt.Sprintf("serve-%d-%s", c.Port, strings.ToLower(string(c.Protocol)))
}

// Spec returns the kube container spec
func (c *ModelContainer) Spec() v1.Container {
	var cmd []string
	switch c.Protocol {
	case TCP:
		cmd = []string{"/agnhost", "serve-hostname", "--tcp", "--http=false", "--port", fmt.Sprintf("%d", c.Port)}
	case UDP:
		cmd = []string{"/agnhost", "serve-hostname", "--udp", "--http=false", "--port", fmt.Sprintf("%d", c.Port)}
	case HTTP:
		cmd = []string{"/agnhost", "serve-hostname", "--http=true", "--port", fmt.Sprintf("%d", c.Port)}
	}

	return v1.Container{
		Name:            c.Name(),
		ImagePullPolicy: v1.PullIfNotPresent,
		Image:           agnHostImage,
		Command:         cmd,
		Env:             []v1.EnvVar{},
		SecurityContext: &v1.SecurityContext{},
		Ports: []v1.ContainerPort{
			{
				ContainerPort: c.Port,
				Name:          c.PortName(),
				Protocol:      c.Protocol.ToV1Proto(),
			},
		},
	}
}
