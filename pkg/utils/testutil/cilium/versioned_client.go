//go:unit

package ciliumutil

import (
	"github.com/sirupsen/logrus"

	"k8s.io/client-go/rest"

	v2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	"github.com/cilium/cilium/pkg/k8s/client/clientset/versioned"
	ciliumv2 "github.com/cilium/cilium/pkg/k8s/client/clientset/versioned/typed/cilium.io/v2"
	ciliumv2alpha1 "github.com/cilium/cilium/pkg/k8s/client/clientset/versioned/typed/cilium.io/v2alpha1"
	discovery "k8s.io/client-go/discovery"
)

// ensure all interfaces are implemented
var (
	_ versioned.Interface        = &MockVersionedClient{}
	_ ciliumv2.CiliumV2Interface = &MockCiliumV2Client{}
)

// MockVersionedClient is a mock implementation of versioned.Interface
// Currently it only returns a real value for CiliumV2()
type MockVersionedClient struct {
	l logrus.FieldLogger
	c *MockCiliumV2Client
}

func NewMockVersionedClient(l logrus.FieldLogger, ciliumEndpoints *MockResource[*v2.CiliumEndpoint]) *MockVersionedClient {
	return &MockVersionedClient{
		l: l,
		c: NewMockCiliumV2Client(l, ciliumEndpoints),
	}
}

func (m *MockVersionedClient) Discovery() discovery.DiscoveryInterface {
	m.l.Warn("MockVersionedClient.Discovery() called but this returns nil because it's not implemented")
	return nil
}

func (m *MockVersionedClient) CiliumV2() ciliumv2.CiliumV2Interface {
	m.l.Info("MockVersionedClient.CiliumV2() called")
	return m.c
}

func (m *MockVersionedClient) CiliumV2alpha1() ciliumv2alpha1.CiliumV2alpha1Interface {
	m.l.Warn("MockVersionedClient.CiliumV2alpha1() called but this returns nil because it's not implemented")
	return nil
}

// MockCiliumV2Client is a mock implementation of ciliumv2.CiliumV2Interface.
// Currently it only returns a real value for CiliumIdentities()
type MockCiliumV2Client struct {
	l               logrus.FieldLogger
	identitiyClient *MockIdentityClient
	ciliumEndpoints *MockResource[*v2.CiliumEndpoint]
}

func NewMockCiliumV2Client(l logrus.FieldLogger, ciliumEndpoints *MockResource[*v2.CiliumEndpoint]) *MockCiliumV2Client {
	return &MockCiliumV2Client{
		l:               l,
		identitiyClient: NewMockIdentityClient(l),
		ciliumEndpoints: ciliumEndpoints,
	}
}

func (m *MockCiliumV2Client) RESTClient() rest.Interface {
	m.l.Warn("MockCiliumV2Client.RESTClient() called but this returns nil because it's not implemented")
	return nil
}

func (m *MockCiliumV2Client) CiliumClusterwideEnvoyConfigs() ciliumv2.CiliumClusterwideEnvoyConfigInterface {
	m.l.Warn("MockCiliumV2Client.CiliumClusterwideEnvoyConfigs() called but this returns nil because it's not implemented")
	return nil
}

func (m *MockCiliumV2Client) CiliumClusterwideNetworkPolicies() ciliumv2.CiliumClusterwideNetworkPolicyInterface {
	m.l.Warn("MockCiliumV2Client.CiliumClusterwideNetworkPolicies() called but this returns nil because it's not implemented")
	return nil
}

func (m *MockCiliumV2Client) CiliumEgressGatewayPolicies() ciliumv2.CiliumEgressGatewayPolicyInterface {
	m.l.Warn("MockCiliumV2Client.CiliumEgressGatewayPolicies() called but this returns nil because it's not implemented")
	return nil
}

func (m *MockCiliumV2Client) CiliumEndpoints(namespace string) ciliumv2.CiliumEndpointInterface {
	m.l.Info("MockCiliumV2Client.CiliumEndpoints() called")
	return NewMockEndpointClient(m.l, namespace, m.ciliumEndpoints)
}

func (m *MockCiliumV2Client) CiliumEnvoyConfigs(_ string) ciliumv2.CiliumEnvoyConfigInterface {
	m.l.Warn("MockCiliumV2Client.CiliumEnvoyConfigs() called but this returns nil because it's not implemented")
	return nil
}

func (m *MockCiliumV2Client) CiliumExternalWorkloads() ciliumv2.CiliumExternalWorkloadInterface {
	m.l.Warn("MockCiliumV2Client.CiliumExternalWorkloads() called but this returns nil because it's not implemented")
	return nil
}

func (m *MockCiliumV2Client) CiliumIdentities() ciliumv2.CiliumIdentityInterface {
	m.l.Info("MockCiliumV2Client.CiliumIdentities() called")
	return m.identitiyClient
}

func (m *MockCiliumV2Client) CiliumLocalRedirectPolicies(_ string) ciliumv2.CiliumLocalRedirectPolicyInterface {
	m.l.Warn("MockCiliumV2Client.CiliumLocalRedirectPolicies() called but this returns nil because it's not implemented")
	return nil
}

func (m *MockCiliumV2Client) CiliumNetworkPolicies(_ string) ciliumv2.CiliumNetworkPolicyInterface {
	m.l.Warn("MockCiliumV2Client.CiliumNetworkPolicies() called but this returns nil because it's not implemented")
	return nil
}

func (m *MockCiliumV2Client) CiliumNodes() ciliumv2.CiliumNodeInterface {
	m.l.Warn("MockCiliumV2Client.CiliumNodes() called but this returns nil because it's not implemented")
	return nil
}

func (m *MockCiliumV2Client) CiliumNodeConfigs(_ string) ciliumv2.CiliumNodeConfigInterface {
	m.l.Warn("MockCiliumV2Client.CiliumNodeConfigs() called but this returns nil because it's not implemented")
	return nil
}
