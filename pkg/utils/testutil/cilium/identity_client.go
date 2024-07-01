//go:unit

package ciliumutil

import (
	"context"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/sirupsen/logrus"

	v2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	ciliumv2 "github.com/cilium/cilium/pkg/k8s/client/clientset/versioned/typed/cilium.io/v2"
)

// ensure all interfaces are implemented
var _ ciliumv2.CiliumIdentityInterface = &MockIdentityClient{}

// MockIdentityClient is a mock implementation of ciliumv2.CiliumIdentityInterface.
// We only implement what's needed. These methods are used by:
// - CRDBackend within the Allocator within the IdentityManager
// - identitygc cell
type MockIdentityClient struct {
	l logrus.FieldLogger
	// identities maps identity name to identity
	// namespace is irrelevant since identity names must be globally unique numbers
	identities map[string]*v2.CiliumIdentity
	watchers   []watch.Interface
}

func NewMockIdentityClient(l logrus.FieldLogger) *MockIdentityClient {
	return &MockIdentityClient{
		l:          l,
		identities: make(map[string]*v2.CiliumIdentity),
		watchers:   make([]watch.Interface, 0),
	}
}

func (m *MockIdentityClient) GetIdentities() map[string]*v2.CiliumIdentity {
	return m.identities
}

func (m *MockIdentityClient) Create(_ context.Context, ciliumIdentity *v2.CiliumIdentity, _ v1.CreateOptions) (*v2.CiliumIdentity, error) {
	m.l.Info("MockIdentityClient.Create() called")
	if _, ok := m.identities[ciliumIdentity.Name]; ok {
		return nil, ErrAlreadyExists
	}

	m.identities[ciliumIdentity.Name] = ciliumIdentity
	return ciliumIdentity, nil
}

func (m *MockIdentityClient) Update(_ context.Context, ciliumIdentity *v2.CiliumIdentity, _ v1.UpdateOptions) (*v2.CiliumIdentity, error) {
	m.l.Info("MockIdentityClient.Update() called")

	if _, ok := m.identities[ciliumIdentity.Name]; ok {
		m.l.Info("MockIdentityClient.Update() found existing identity")
	} else {
		m.l.Info("MockIdentityClient.Update() did not find existing identity")
	}

	m.identities[ciliumIdentity.Name] = ciliumIdentity
	return ciliumIdentity, nil
}

func (m *MockIdentityClient) Delete(_ context.Context, name string, _ v1.DeleteOptions) error {
	m.l.Info("MockIdentityClient.Delete() called")

	if _, ok := m.identities[name]; ok {
		m.l.Info("MockIdentityClient.Delete() found existing identity")
	} else {
		m.l.Info("MockIdentityClient.Delete() did not find existing identity")
	}

	delete(m.identities, name)
	return nil
}

func (m *MockIdentityClient) DeleteCollection(_ context.Context, _ v1.DeleteOptions, _ v1.ListOptions) error {
	m.l.Warn("MockIdentityClient.DeleteCollection() called but this is not implemented")
	return ErrNotImplemented
}

func (m *MockIdentityClient) Get(_ context.Context, name string, _ v1.GetOptions) (*v2.CiliumIdentity, error) {
	m.l.Info("MockIdentityClient.Get() called")

	if identity, ok := m.identities[name]; ok {
		m.l.Info("MockIdentityClient.Get() found existing identity")
		return identity, nil
	}

	return nil, ErrNotFound{}
}

func (m *MockIdentityClient) List(_ context.Context, _ v1.ListOptions) (*v2.CiliumIdentityList, error) {
	m.l.Info("MockIdentityClient.List() called")

	items := make([]v2.CiliumIdentity, 0, len(m.identities))
	for _, identity := range m.identities {
		items = append(items, *identity)
	}

	return &v2.CiliumIdentityList{Items: items}, nil
}

func (m *MockIdentityClient) Watch(_ context.Context, _ v1.ListOptions) (watch.Interface, error) {
	m.l.Warn("MockIdentityClient.Watch() called but this returns a fake watch because it's not implemented")

	// not sure if watching is important for us
	w := watch.NewFake()
	m.watchers = append(m.watchers, w)
	return w, nil
}

func (m *MockIdentityClient) Patch(_ context.Context, _ string, _ types.PatchType, _ []byte, _ v1.PatchOptions, _ ...string) (result *v2.CiliumIdentity, err error) {
	m.l.Warn("MockIdentityClient.Patch() called but this returns nil because it's not implemented")
	return nil, ErrNotImplemented
}
