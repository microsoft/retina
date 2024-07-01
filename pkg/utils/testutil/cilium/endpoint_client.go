//go:unit

package ciliumutil

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sirupsen/logrus"

	v2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	ciliumv2 "github.com/cilium/cilium/pkg/k8s/client/clientset/versioned/typed/cilium.io/v2"
	"github.com/cilium/cilium/pkg/k8s/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
)

// ensure all interfaces are implemented
var _ ciliumv2.CiliumEndpointInterface = &MockEndpointClient{}

type MockEndpointClient struct {
	l               logrus.FieldLogger
	namespace       string
	ciliumEndpoints *MockResource[*v2.CiliumEndpoint]
	watchers        []watch.Interface
}

func NewMockEndpointClient(l logrus.FieldLogger, namespace string, ciliumEndpoints *MockResource[*v2.CiliumEndpoint]) *MockEndpointClient {
	return &MockEndpointClient{
		l:               l,
		namespace:       namespace,
		ciliumEndpoints: ciliumEndpoints,
		watchers:        make([]watch.Interface, 0),
	}
}

func (m *MockEndpointClient) Create(_ context.Context, ciliumEndpoint *v2.CiliumEndpoint, _ v1.CreateOptions) (*v2.CiliumEndpoint, error) {
	m.l.Info("MockEndpointClient.Create() called")
	_, ok, err := m.ciliumEndpoints.GetByKey(resource.NewKey(ciliumEndpoint))
	if err != nil {
		return nil, err
	}
	if ok {
		return nil, ErrAlreadyExists
	}

	m.ciliumEndpoints.Upsert(ciliumEndpoint)
	return ciliumEndpoint, nil
}

func (m *MockEndpointClient) Update(_ context.Context, ciliumEndpoint *v2.CiliumEndpoint, _ v1.UpdateOptions) (*v2.CiliumEndpoint, error) {
	m.l.Info("MockEndpointClient.Update() called")
	m.ciliumEndpoints.cache[resource.NewKey(ciliumEndpoint)] = ciliumEndpoint
	return ciliumEndpoint, nil
}

func (m *MockEndpointClient) UpdateStatus(_ context.Context, _ *v2.CiliumEndpoint, _ v1.UpdateOptions) (*v2.CiliumEndpoint, error) {
	m.l.Warn("MockEndpointClient.UpdateStatus() called but this returns nil because it's not implemented")
	return nil, ErrNotImplemented
}

func (m *MockEndpointClient) Delete(_ context.Context, name string, _ v1.DeleteOptions) error {
	m.l.Info("MockEndpointClient.Delete() called")
	_, ok, err := m.ciliumEndpoints.GetByKey(resource.Key{Name: name, Namespace: m.namespace})
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound{}
	}
	m.ciliumEndpoints.Delete(resource.Key{Name: name, Namespace: m.namespace})
	return nil
}

func (m *MockEndpointClient) DeleteCollection(_ context.Context, _ v1.DeleteOptions, _ v1.ListOptions) error {
	m.l.Warn("MockEndpointClient.DeleteCollection() called but this is not implemented")
	return ErrNotImplemented
}

func (m *MockEndpointClient) Get(_ context.Context, name string, _ v1.GetOptions) (*v2.CiliumEndpoint, error) {
	m.l.Info("MockEndpointClient.Get() called")
	item, _, err := m.ciliumEndpoints.GetByKey(resource.Key{Name: name, Namespace: m.namespace})
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (m *MockEndpointClient) List(_ context.Context, _ v1.ListOptions) (*v2.CiliumEndpointList, error) {
	m.l.Info("MockEndpointClient.List() called")

	items := make([]v2.CiliumEndpoint, 0, len(m.ciliumEndpoints.cache))
	for _, cep := range m.ciliumEndpoints.cache {
		items = append(items, *cep)
	}

	return &v2.CiliumEndpointList{Items: items}, nil
}

func (m *MockEndpointClient) Watch(_ context.Context, _ v1.ListOptions) (watch.Interface, error) {
	m.l.Warn("MockEndpointClient.Watch() called but this returns a fake watch because it's not implemented")

	// not sure if watching is important for us
	w := watch.NewFake()
	m.watchers = append(m.watchers, w)
	return w, nil
}

func (m *MockEndpointClient) Patch(_ context.Context, name string, _ types.PatchType, data []byte, _ v1.PatchOptions, _ ...string) (result *v2.CiliumEndpoint, err error) {
	key := resource.Key{Name: name, Namespace: m.namespace}
	cep, ok, err := m.ciliumEndpoints.GetByKey(key)
	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, ErrNotFound{}
	}

	var replaceCEPStatus []JSONPatch
	err = json.Unmarshal(data, &replaceCEPStatus)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal patch data: %w", err)
	}

	cep.Status = replaceCEPStatus[0].Value
	m.ciliumEndpoints.Upsert(cep)
	cep, ok, err = m.ciliumEndpoints.GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound{}
	}

	return cep, nil
}

type JSONPatch struct {
	OP    string            `json:"op,omitempty"`
	Path  string            `json:"path,omitempty"`
	Value v2.EndpointStatus `json:"value"`
}
