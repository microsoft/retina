// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Hubble

package ebpfwindows

import (
	"log/slog"
	"net/netip"

	pb "github.com/cilium/cilium/api/v1/flow"
	"github.com/cilium/cilium/pkg/logging"
	"github.com/cilium/cilium/pkg/time"
)

type DatapathContext struct {
	SrcIP                 netip.Addr
	SrcLabelID            uint32
	DstIP                 netip.Addr
	DstLabelID            uint32
	TraceObservationPoint pb.TraceObservationPoint
}

type EndpointResolver struct {
	log        *slog.Logger
	logLimiter logging.Limiter
}

func NewEndpointResolver(
	log *slog.Logger,
) *EndpointResolver {
	return &EndpointResolver{
		log:        log,
		logLimiter: logging.NewLimiter(30*time.Second, 1),
	}
}

func (r *EndpointResolver) ResolveEndpoint(_ netip.Addr, datapathSecurityIdentity uint32, _ DatapathContext) *pb.Endpoint {
	// for remote endpoints, assemble the information via ip and identity
	numericIdentity := datapathSecurityIdentity
	var namespace, podName string
	var labels []string
	var clusterName string

	return &pb.Endpoint{
		Identity:    numericIdentity,
		ClusterName: clusterName,
		Namespace:   namespace,
		Labels:      labels,
		PodName:     podName,
	}
}
