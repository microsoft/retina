package common

import (
	"net/netip"
	"os"

	"github.com/cilium/cilium/api/v1/flow"
	"github.com/cilium/cilium/pkg/identity"
	ipc "github.com/cilium/cilium/pkg/ipcache"
	"github.com/cilium/cilium/pkg/k8s"
	"github.com/cilium/cilium/pkg/labels"
)

//go:generate go run github.com/golang/mock/mockgen@v1.6.0 -source decoder.go -destination=mocks/mock_types.go -package=mocks

type EpDecoder interface {
	Decode(ip netip.Addr) *flow.Endpoint
	IsEndpointOnLocalHost(ip string) bool
}

type epDecoder struct {
	localHostIP string
	ipcache     *ipc.IPCache
}

func NewEpDecoder(c *ipc.IPCache) EpDecoder {
	return &epDecoder{
		localHostIP: os.Getenv("NODE_IP"),
		ipcache:     c,
	}
}

func (e *epDecoder) Decode(ip netip.Addr) *flow.Endpoint {
	ep := &flow.Endpoint{}
	if metadata := e.ipcache.GetK8sMetadata(ip); metadata != nil {
		ep.PodName = metadata.PodName
		ep.Namespace = metadata.Namespace
	}
	id, ok := e.ipcache.LookupByIP(ip.String())
	if !ok {
		// Default to world.
		id = ipc.Identity{ID: identity.ReservedIdentityWorld}
	}
	ep.ID = id.ID.Uint32()
	ep.Identity = id.ID.Uint32()

	switch id.ID { //nolint:exhaustive // We don't need all the cases.
	case identity.ReservedIdentityHost:
		ep.Labels = labels.LabelHost.GetModel()
	case identity.ReservedIdentityKubeAPIServer:
		ep.Labels = labels.LabelKubeAPIServer.GetModel()
	case identity.ReservedIdentityRemoteNode:
		ep.Labels = labels.LabelRemoteNode.GetModel()
	case identity.ReservedIdentityWorld:
		ep.Labels = labels.LabelWorld.GetModel()
	default:
		// TODO: We do not have an api on the ipcache to get the labels from the ip or identity.
	}

	return ep
}

func (e *epDecoder) IsEndpointOnLocalHost(string) bool {
	// TODO: We need to check if the ip is in the local host network.
	// We need the ipcache to provide an api for the same.
	return false
}

type SvcDecoder interface {
	Decode(ip netip.Addr) *flow.Service
}

type svcDecoder struct {
	svccache k8s.ServiceCache
}

func NewSvcDecoder(sc k8s.ServiceCache) SvcDecoder {
	return &svcDecoder{
		svccache: sc,
	}
}

func (s *svcDecoder) Decode(netip.Addr) *flow.Service {
	svc := &flow.Service{}
	// TODO: serviceCache from cilium do not have a way to get the service name
	// and namespace from the ip. We need to add this to the serviceCache.
	return svc
}
