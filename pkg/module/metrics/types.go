// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package metrics

import (
	"fmt"
	"net"
	"strings"

	"github.com/cilium/cilium/api/v1/flow"
	api "github.com/microsoft/retina/crd/api/v1alpha1"
	"github.com/microsoft/retina/pkg/common"
	"github.com/microsoft/retina/pkg/utils"
)

const (
	// source context options prefix
	sourceCtxPrefix = "source_"

	// destination context options prefix
	destinationCtxPrefix = "destination_"

	// IP Context Option
	ipCtxOption = "ip"

	// namespace context option
	namespaceCtxOption = "namespace"

	// pod context option
	podCtxOption = "podname"

	// service context option
	serviceCtxOption = "service"

	// port context option
	portCtxOption = "port"

	// workloads context option
	workloadKindCtxOption = "workload_kind"

	// workload context option
	workloadNameCtxOption = "workload_name"

	// workload context option
	workloadCtxOption = "workload"

	// zone context option
	zoneCtxOption = "zone"

	// localContext means only the pods on this node will be watched
	// and only these events will be enriched
	localContext enrichmentContext = "local"

	// remoteContext means all pods on the cluster will be watched
	// and events will be enriched
	remoteContext enrichmentContext = "remote"

	// ingress means the direction of the flow is from outside the pod to inside
	ingress = "ingress"

	// egress means the direction of the flow is from inside the pod to outside
	egress = "egress"
)

//go:generate go run go.uber.org/mock/mockgen@v0.4.0 -source=types.go -destination=mock_types.go -package=metrics
type IModule interface {
	Reconcile(spec *api.MetricsSpec) error
}

type enrichmentContext string

type ctxOptionType int

const (
	source ctxOptionType = iota + 1
	destination
	localCtx
)

type AdvMetricsInterface interface {
	Init(metricName string)
	// This func is used to clean up old metrics on reconcile.
	Clean()
	ProcessFlow(f *flow.Flow)
}

type ContextOptionsInterface interface {
	getLabels() []string
	getValues(f *flow.Flow) []string
	getLocalCtxValues(f *flow.Flow) map[string][]string
}

type ContextOptions struct {
	option    ctxOptionType
	IP        bool
	Namespace bool
	Podname   bool
	Workload  bool
	Service   bool
	Port      bool
	Zone      bool
}

type DirtyCachePod struct {
	// IP is the IP of the endpoint
	IP net.IP
	// indicates if pod is annotated / of interest
	Annotated bool
	// indicates if pod namespace is annotated / of interest
	Namespaced bool
}

func NewCtxOption(opts []string, option ctxOptionType) *ContextOptions {
	c := &ContextOptions{
		option: option,
	}

	if opts == nil {
		return c
	}
	// TODO check lower case here
	for _, opt := range opts {
		switch strings.ToLower(opt) {
		case ipCtxOption:
			c.IP = true
		case namespaceCtxOption:
			c.Namespace = true
		case podCtxOption:
			c.Podname = true
		case workloadCtxOption:
			c.Workload = true
		case serviceCtxOption:
			c.Service = true
		case portCtxOption:
			c.Port = true
		case zoneCtxOption:
			c.Zone = true
		}
	}

	return c
}

func (c *ContextOptions) getLabels() []string {
	// Note: order of append here of labels should match the order of values
	prefix := ""
	switch c.option {
	case source:
		prefix = sourceCtxPrefix
	case destination:
		prefix = destinationCtxPrefix
	case localCtx:
		// no prefix for localcontext as the aggregation is at each pod level
		prefix = ""
	}

	labels := make([]string, 0)
	if c.IP {
		labels = append(labels, prefix+ipCtxOption)
	}
	if c.Namespace {
		labels = append(labels, prefix+namespaceCtxOption)
	}
	if c.Podname {
		labels = append(labels, prefix+podCtxOption)
	}
	if c.Workload {
		labels = append(labels, prefix+workloadKindCtxOption, prefix+workloadNameCtxOption)
	}

	if c.Service {
		labels = append(labels, prefix+serviceCtxOption)
	}

	if c.Port {
		labels = append(labels, prefix+portCtxOption)
	}

	if c.Zone {
		labels = append(labels, prefix+zoneCtxOption)
	}

	return labels
}

func (c *ContextOptions) getValues(f *flow.Flow) []string {
	return c.getByDirectionValues(f, c.isDest())
}

func (c *ContextOptions) isDest() bool {
	return c.option == destination
}

func (c *ContextOptions) isSrc() bool {
	return c.option == source
}

func (c *ContextOptions) getLocalCtxValues(f *flow.Flow) map[string][]string {
	values := map[string][]string{
		ingress: nil,
		egress:  nil,
	}
	if c.option != localCtx {
		return nil
	}

	// for local context, we only enrich the information of pods running on the same node
	// as retina pods. So if a src/dst field is non empty then we will need to
	// add a metric specific to that pod based on the direction.
	// there will be a case where src and dst are both filled, in that case
	// we will need to add two metrics, one for src and one for dst based on direction
	// and location of observation i.e. tracepoint
	//
	// if only Source pod info is available:
	// 			- return labels of this pod with direction as EGRESS
	// if only Destination pod info is available:
	// 			- return labels of this pod with direction as INGRESS
	// if both Source and Destination pod info is available:
	// 			- return labels of source pod with direction as EGRESS
	// 			- return labels of destination pod with direction as INGRESS

	if f == nil {
		return values
	}

	if f.Source != nil && !isAPIServerPod(f.Source) {
		values[egress] = c.getByDirectionValues(f, false)
	}

	if f.Destination != nil && !isAPIServerPod(f.Destination) {
		values[ingress] = c.getByDirectionValues(f, true)
	}

	return values
}

func (c *ContextOptions) getByDirectionValues(f *flow.Flow, dest bool) []string {
	// Note: order of append here of values should match the order of labels
	values := make([]string, 0)
	if f == nil {
		return values
	}

	if c.IP {
		ip := "unknown"
		if f.IP != nil {
			ip = f.IP.Source
			if dest {
				ip = f.IP.Destination
			}
		}
		values = append(values, ip)
	}

	ep := f.Source
	if dest {
		ep = f.Destination
	}

	if c.Namespace {
		if ep != nil {
			values = append(values, ep.Namespace)
		} else {
			values = append(values, "unknown")
		}
	}

	if c.Podname {
		if ep != nil {
			values = append(values, ep.PodName)
		} else {
			values = append(values, "unknown")
		}
	}

	if c.Workload {
		wk := ep.GetWorkloads()
		if len(wk) > 0 {
			values = append(values, wk[0].Kind, wk[0].Name)
		} else {
			values = append(values, "unknown", "unknown")
		}
	}

	if c.Service {
		if dest {
			if f.DestinationService != nil {
				values = append(values, f.DestinationService.Name)
			} else {
				values = append(values, "unknown")
			}
		} else {
			if f.SourceService != nil {
				values = append(values, f.SourceService.Name)
			} else {
				values = append(values, "unknown")
			}
		}
	}

	if c.Port {
		l4 := f.GetL4()
		if l4 != nil {
			// check if TCP object is present
			if tcp := l4.GetTCP(); tcp != nil {
				if dest {
					values = append(values, fmt.Sprintf("%d", tcp.GetDestinationPort()))
				} else {
					values = append(values, fmt.Sprintf("%d", tcp.GetSourcePort()))
				}
			} else if udp := l4.GetUDP(); udp != nil {
				if dest {
					values = append(values, fmt.Sprintf("%d", udp.GetDestinationPort()))
				} else {
					values = append(values, fmt.Sprintf("%d", udp.GetSourcePort()))
				}
			}
		} else {
			values = append(values, "unknown")
		}
	}

	if c.Zone {
		// NodeLabels is only the source node
		if !dest {
			values = append(values, utils.SourceZone(f))
		} else {
			values = append(values, utils.DestinationZone(f))
		}
	}

	return values
}

// DefaultCtxOptions used for enableAnnotations where it sets the source and destination labels
// so users will not have to manually define.
func DefaultCtxOptions() []string {
	return []string{
		ipCtxOption,
		namespaceCtxOption,
		podCtxOption,
		workloadCtxOption,
		// ignoring service option as we have not added right logic around it
		// TODO add service specific enrichment and logic, #610
		// serviceCtxOption,
		// Port adds a ton of extra dimension to metrics
		// so not adding it as a default option
		// portCtxOption,
	}
}

// DefaultMetrics used for enableAnnotations where it sets enabled advanced metrics
// so users will not have to manually define.
// For any new advanced metrics we want to have enabled by default for annotation based solution, add it here.
func DefaultMetrics() []string {
	return []string{
		// forward
		utils.ForwardPacketsGaugeName,
		utils.ForwardBytesGaugeName,
		// drop
		utils.DroppedPacketsGaugeName,
		utils.DropBytesGaugeName,
		// tcp flags
		utils.TCPFlagGauge,
		// tcp retransmissions
		utils.TCPRetransCount,
		// latency
		utils.NodeAPIServerLatencyName,
		utils.NodeAPIServerTCPHandshakeLatencyName,
		utils.NoResponseFromAPIServerName,
		// dns
		utils.DNSRequestCounterName,
		utils.DNSResponseCounterName,
	}
}

func isAPIServerPod(ep *flow.Endpoint) bool {
	if ep == nil {
		return false
	}

	if ep.Namespace == common.APIServerEndpointName && ep.PodName == common.APIServerEndpointName {
		return true
	}

	return false
}
