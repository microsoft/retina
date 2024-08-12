package netpolagent

import (
	"fmt"
	"sync"

	slim_networkingv1 "github.com/cilium/cilium/pkg/k8s/slim/k8s/api/networking/v1"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/network-policy-api/cmd/policy-assistant/pkg/matcher"

	"github.com/cilium/cilium/pkg/k8s/resource"
)

type PolicyMetadata struct {
	Name      string
	Namespace string
	Kind      string
}

type store struct {
	sync.Mutex
	l               logrus.FieldLogger
	allPolicies     *matcher.Policy
	NetworkPolicies map[resource.Key]*slim_networkingv1.NetworkPolicy
}

func newStore(l logrus.FieldLogger) *store {
	return &store{
		l:               l,
		allPolicies:     matcher.NewPolicy(),
		NetworkPolicies: make(map[resource.Key]*slim_networkingv1.NetworkPolicy),
	}
}

func (s *store) UpsertNPV1(key resource.Key, slim *slim_networkingv1.NetworkPolicy) {
	s.Lock()
	defer s.Unlock()

	if old, ok := s.NetworkPolicies[key]; ok {
		if slim.DeepEqual(old) {
			return
		}

		s.updateNPV1(key, slim)
		return
	}

	s.addNPV1(key, slim)
}

func (s *store) updateNPV1(key resource.Key, slim *slim_networkingv1.NetworkPolicy) {
	s.l.WithField("key", fmt.Sprintf("%s/%s", key.Namespace, key.Name)).Debug("updating existing network policy")

	// have to recalculate all policies
	s.NetworkPolicies[key] = slim
	s.rebuildPolicies()
}

func (s *store) addNPV1(key resource.Key, slim *slim_networkingv1.NetworkPolicy) {
	s.l.WithField("key", fmt.Sprintf("%s/%s", key.Namespace, key.Name)).Debug("adding new network policy")

	s.NetworkPolicies[key] = slim
	npv1 := slimToNPV1(slim)[0]
	ingress, egress := matcher.BuildTarget(npv1)
	s.allPolicies.AddTarget(true, ingress)
	s.allPolicies.AddTarget(false, egress)
}

func (s *store) DeleteNPV1(key resource.Key) {
	s.Lock()
	defer s.Unlock()

	if _, ok := s.NetworkPolicies[key]; !ok {
		return
	}

	s.l.WithField("key", fmt.Sprintf("%s/%s", key.Namespace, key.Name)).Debug("deleting network policy")
	delete(s.NetworkPolicies, key)

	// have to recalculate all policies
	s.rebuildPolicies()
}

func (s *store) rebuildPolicies() {
	slims := make([]*slim_networkingv1.NetworkPolicy, 0, len(s.NetworkPolicies))
	for _, np := range s.NetworkPolicies {
		slims = append(slims, np)
	}
	nps := slimToNPV1(slims...)
	s.allPolicies = matcher.BuildNetworkPolicies(false, nps)
}
