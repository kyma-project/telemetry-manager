package k8s

import (
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type NetworkPolicy struct {
	name      string
	namespace string
}

func NewNetworkPolicy(name, namespace string) *NetworkPolicy {
	networkPolicy := &NetworkPolicy{
		name:      name,
		namespace: namespace,
	}

	return networkPolicy
}

func (n *NetworkPolicy) K8sObject() *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      n.name,
			Namespace: n.namespace,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
			},
		},
	}
}
