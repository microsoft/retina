package netpolagent

import (
	"context"
	"strings"

	"github.com/microsoft/retina/pkg/networkpolicy"

	"sigs.k8s.io/network-policy-api/cmd/policy-assistant/pkg/matcher"

	"github.com/cilium/cilium/api/v1/flow"
	"github.com/cilium/cilium/pkg/hive/cell"
	"github.com/cilium/cilium/pkg/k8s/resource"
	slim_networkingv1 "github.com/cilium/cilium/pkg/k8s/slim/k8s/api/networking/v1"
	"github.com/cilium/cilium/pkg/labels"
	"github.com/cilium/workerpool"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	maxWorkers = 2
	// equal to PodNamespaceMetaLabels (from github.com/cilium/cilium/pkg/k8s/apis/cilium.io) + PathDelimiter (from github.com/cilium/cilium/pkg/labels)
	namespaceMetaLabelsPrefix = "io.cilium.k8s.namespace.labels."
)

var labelPrefixesToIgnore = []string{
	"io.cilium.",
	"io.kubernetes.",
}

type agentParams struct {
	cell.In

	Lifecycle cell.Lifecycle
	Log       logrus.FieldLogger
	Config    networkpolicy.Config
	npv1      resource.Resource[*slim_networkingv1.NetworkPolicy]
}

type NetPolAgent struct {
	l       logrus.FieldLogger
	enabled bool
	npv1    resource.Resource[*slim_networkingv1.NetworkPolicy]
	store   *store
	wp      *workerpool.WorkerPool
}

func newNetPolAgent(p agentParams) *NetPolAgent {
	l := p.Log.WithField("component", "networkpolicy-agent")

	if !p.Config.EnableNetworkPolicyEnrichment {
		n := &NetPolAgent{
			l:       l,
			enabled: false,
		}

		return n
	}

	n := &NetPolAgent{
		l:       l,
		enabled: true,
		npv1:    p.npv1,
		store:   newStore(l),
		wp:      workerpool.New(maxWorkers),
	}

	p.Lifecycle.Append(n)

	return n
}

func (n *NetPolAgent) Start(_ cell.HookContext) error {
	if err := n.wp.Submit("npv1-controller", n.runNPV1Controller); err != nil {
		return errors.Wrap(err, "failed to submit npv1-controller")
	}

	return nil
}

func (n *NetPolAgent) Stop(_ cell.HookContext) error {
	if err := n.wp.Close(); err != nil {
		return errors.Wrap(err, "failed to stop workerpool")
	}

	return nil
}

// PoliciesDroppingTraffic returns the policies that are causing traffic to be dropped.
// The first list is policies impacting ingress, the second list is policies impacting egress.
// Only NetworkPolicyV1 is supported currently.
func (n *NetPolAgent) PoliciesDroppingTraffic(src, dst *flow.Endpoint) ([]*PolicyMetadata, []*PolicyMetadata) {
	if !n.enabled || src == nil || dst == nil {
		return nil, nil
	}

	traffic := &matcher.Traffic{
		Source:      endpointToTraffic(src),
		Destination: endpointToTraffic(dst),
	}

	ingress := n.policiesDroppingTraffic(traffic, true)
	egress := n.policiesDroppingTraffic(traffic, false)
	return ingress, egress
}

func (n *NetPolAgent) policiesDroppingTraffic(traffic *matcher.Traffic, isIngress bool) []*PolicyMetadata {
	// NOTE: copied/modified from matcher.Policy.IsIngressOrEgressAllowed()

	var subject *matcher.TrafficPeer
	var peer *matcher.TrafficPeer
	if isIngress {
		subject = traffic.Destination
		peer = traffic.Source
	} else {
		subject = traffic.Source
		peer = traffic.Destination
	}

	// 1. if target is external to cluster -> allow
	//   this is because we can't stop external hosts from sending or receiving traffic
	if subject.Internal == nil {
		return nil
	}

	n.store.Lock()
	matchingTargets := n.store.allPolicies.TargetsApplyingToPod(isIngress, subject.Internal)
	n.store.Unlock()

	// 2. No targets match => automatic allow
	if len(matchingTargets) == 0 {
		return nil
	}

	// 3. Check if any matching targets allow this traffic
	for _, target := range matchingTargets {
		for _, m := range target.Peers {
			if m.Matches(subject, peer, traffic.ResolvedPort, traffic.ResolvedPortName, traffic.Protocol) {
				return nil
			}
		}
	}

	policies := make([]*PolicyMetadata, 0)
	for _, target := range matchingTargets {
		for _, r := range target.SourceRules {
			split := strings.Split(string(r), "/")
			if len(split) != 2 {
				n.l.WithField("key", string(r)).Warn("invalid policy key from policy-assistant")
				policies = append(policies, &PolicyMetadata{
					Name: string(r),
				})
				continue
			}

			policies = append(policies, &PolicyMetadata{
				Name:      split[0],
				Namespace: split[1],
			})
		}
	}

	return policies
}

func (n *NetPolAgent) runNPV1Controller(ctx context.Context) error {
	n.l.Info("start to reconcile npv1")

	npv1Events := n.npv1.Events(ctx)

	for {
		select {
		case ev, ok := <-npv1Events:
			if !ok {
				n.l.Info("npv1 events channel is closed. stopping reconciling npv1")
				return nil
			}

			var err error
			switch ev.Kind {
			case resource.Sync:
				// ignore
			case resource.Upsert:
				n.reconcileNPV1(ev.Key, ev.Object)
			case resource.Delete:
				n.store.DeleteNPV1(ev.Key)
			}

			if err != nil {
				n.l.WithError(err).WithField("namespaceKey", ev.Key.String()).Error("error creating cilium endpoint. requeuing namespace")
			}
			ev.Done(err)
		case <-ctx.Done():
			n.l.Info("stop reconciling npv1")
			return nil
		}
	}
}

func (n *NetPolAgent) reconcileNPV1(key resource.Key, slim *slim_networkingv1.NetworkPolicy) {
	if slim == nil || slim.DeletionTimestamp != nil {
		// the policy has been deleted
		n.store.DeleteNPV1(key)
		return
	}

	n.store.UpsertNPV1(key, slim)
}

func endpointToTraffic(ep *flow.Endpoint) *matcher.TrafficPeer {
	if ep == nil {
		return nil
	}

	lbls := labels.NewLabelsFromModel(ep.Labels)
	podLabels := make(map[string]string)
	nsLabels := make(map[string]string)
	for _, lbl := range lbls {
		if lbl.Source != labels.LabelSourceK8s {
			continue
		}

		if strings.HasPrefix(lbl.Key, namespaceMetaLabelsPrefix) {
			nsKey := strings.TrimPrefix(lbl.Key, namespaceMetaLabelsPrefix)
			nsLabels[nsKey] = lbl.Value
			continue
		}

		for _, prefix := range labelPrefixesToIgnore {
			if strings.HasPrefix(lbl.Key, prefix) {
				continue
			}
		}

		podLabels[lbl.Key] = lbl.Value
	}

	// assume all traffic is Pod to Pod right now for simplicity
	// FIXME handle more^ (use ID and check if it's a reserved identity like in pkg/hubble/common/decoder_linux.go)

	return &matcher.TrafficPeer{
		Internal: &matcher.InternalPeer{
			PodLabels:       podLabels,
			NamespaceLabels: nsLabels,
			Namespace:       ep.Namespace,
		},
		// TODO is this field supported? Or should we use Internal.Pods? Is that supported?
		// once supported we should get the Pod's IP here for proper analysis with CIDR blocks
		// IP:       ip,
	}
}
