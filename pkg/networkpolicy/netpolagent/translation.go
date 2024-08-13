package netpolagent

import (
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	slim_networkingv1 "github.com/cilium/cilium/pkg/k8s/slim/k8s/api/networking/v1"
	slim_metav1 "github.com/cilium/cilium/pkg/k8s/slim/k8s/apis/meta/v1"
	ciliumintstr "github.com/cilium/cilium/pkg/k8s/slim/k8s/apis/util/intstr"
)

func slimToNPV1(slim ...*slim_networkingv1.NetworkPolicy) []*networkingv1.NetworkPolicy {
	if slim == nil {
		return nil
	}
	nps := make([]*networkingv1.NetworkPolicy, 0, len(slim))
	for _, s := range slim {
		np := &networkingv1.NetworkPolicy{
			TypeMeta: metav1.TypeMeta{
				Kind:       s.Kind,
				APIVersion: s.APIVersion,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      s.ObjectMeta.Name,
				Namespace: s.ObjectMeta.Namespace,
			},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: *convertSlimLabelSelector(&s.Spec.PodSelector),
				Ingress:     convertSlimIngressRules(s.Spec.Ingress),
				Egress:      convertSlimEgressRules(s.Spec.Egress),
				PolicyTypes: convertSlimPolicyTypes(s.Spec.PolicyTypes),
			},
		}
		nps = append(nps, np)
	}
	return nps
}

func convertSlimLabelSelectorRequirements(slim []slim_metav1.LabelSelectorRequirement) []metav1.LabelSelectorRequirement {
	if slim == nil {
		return nil
	}
	reqs := make([]metav1.LabelSelectorRequirement, len(slim))
	for i, s := range slim {
		reqs[i] = metav1.LabelSelectorRequirement{
			Key:      s.Key,
			Operator: metav1.LabelSelectorOperator(s.Operator),
			Values:   s.Values,
		}
	}
	return reqs
}

func convertSlimIngressRules(slim []slim_networkingv1.NetworkPolicyIngressRule) []networkingv1.NetworkPolicyIngressRule {
	if slim == nil {
		return nil
	}
	rules := make([]networkingv1.NetworkPolicyIngressRule, len(slim))
	for i, s := range slim {
		rules[i] = networkingv1.NetworkPolicyIngressRule{
			Ports: convertSlimNetworkPolicyPorts(s.Ports),
			From:  convertSlimNetworkPolicyPeers(s.From),
		}
	}
	return rules
}

func convertSlimEgressRules(slim []slim_networkingv1.NetworkPolicyEgressRule) []networkingv1.NetworkPolicyEgressRule {
	if slim == nil {
		return nil
	}
	rules := make([]networkingv1.NetworkPolicyEgressRule, len(slim))
	for i, s := range slim {
		rules[i] = networkingv1.NetworkPolicyEgressRule{
			Ports: convertSlimNetworkPolicyPorts(s.Ports),
			To:    convertSlimNetworkPolicyPeers(s.To),
		}
	}
	return rules
}

func convertSlimNetworkPolicyPorts(slim []slim_networkingv1.NetworkPolicyPort) []networkingv1.NetworkPolicyPort {
	if slim == nil {
		return nil
	}
	ports := make([]networkingv1.NetworkPolicyPort, len(slim))
	for i, s := range slim {
		ports[i] = networkingv1.NetworkPolicyPort{
			Protocol: (*v1.Protocol)(s.Protocol),
			Port:     convertSlimIntOrString(s.Port),
			EndPort:  s.EndPort,
		}
	}
	return ports
}

func convertSlimIntOrString(slim *ciliumintstr.IntOrString) *intstr.IntOrString {
	if slim == nil {
		return nil
	}

	return &intstr.IntOrString{
		Type:   intstr.Type(slim.Type),
		IntVal: slim.IntVal,
		StrVal: slim.StrVal,
	}
}

func convertSlimNetworkPolicyPeers(slim []slim_networkingv1.NetworkPolicyPeer) []networkingv1.NetworkPolicyPeer {
	if slim == nil {
		return nil
	}
	peers := make([]networkingv1.NetworkPolicyPeer, len(slim))
	for i, s := range slim {
		peers[i] = networkingv1.NetworkPolicyPeer{
			PodSelector:       convertSlimLabelSelector(s.PodSelector),
			NamespaceSelector: convertSlimLabelSelector(s.NamespaceSelector),
			IPBlock:           convertSlimIPBlock(s.IPBlock),
		}
	}
	return peers
}

func convertSlimLabelSelector(slim *slim_metav1.LabelSelector) *metav1.LabelSelector {
	if slim == nil {
		return nil
	}
	return &metav1.LabelSelector{
		MatchLabels:      slim.MatchLabels,
		MatchExpressions: convertSlimLabelSelectorRequirements(slim.MatchExpressions),
	}
}

func convertSlimIPBlock(slim *slim_networkingv1.IPBlock) *networkingv1.IPBlock {
	if slim == nil {
		return nil
	}
	return &networkingv1.IPBlock{
		CIDR:   slim.CIDR,
		Except: slim.Except,
	}
}

func convertSlimPolicyTypes(slim []slim_networkingv1.PolicyType) []networkingv1.PolicyType {
	if slim == nil {
		return nil
	}
	types := make([]networkingv1.PolicyType, len(slim))
	for i, s := range slim {
		types[i] = networkingv1.PolicyType(s)
	}
	return types
}

// func npv1ListToNPV1(slim []*slim_networkingv1.NetworkPolicy) []*networkingv1.NetworkPolicy {
// 	if slim == nil {
// 		return nil
// 	}

// 	nps := make([]*networkingv1.NetworkPolicy, 0, len(slim))
// 	for _, s := range slim {
// 		np := &networkingv1.NetworkPolicy{
// 			ObjectMeta: metav1.ObjectMeta{
// 				Name:      s.Name,
// 				Namespace: s.Namespace,
// 			},
// 			Spec: networkingv1.NetworkPolicySpec{
// 				PodSelector: metav1.LabelSelector{
// 					MatchLabels: s.Spec.PodSelector.MatchLabels,
// 				},
// 				Ingress: s.Spec.Ingress,
// 				// PolicyTypes: s.Spec.PolicyTypes,
// 			},
// 		}

// 		nps = append(nps, np)
// 	}

// 	return nps
// }
