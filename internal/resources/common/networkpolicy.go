package common

import (
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// NetworkPolicyOption is a functional option for configuring a NetworkPolicy
type NetworkPolicyOption func(*networkingv1.NetworkPolicySpec)

// MakeNetworkPolicy creates a NetworkPolicy with the given name, labels, pod selector, and options.
// PolicyTypes are automatically derived based on which rules are present:
// - Ingress type is added only if ingress rules exist
// - Egress type is added only if egress rules exist
func MakeNetworkPolicy(
	name types.NamespacedName,
	labels map[string]string,
	selectorLabels map[string]string,
	opts ...NetworkPolicyOption,
) *networkingv1.NetworkPolicy {
	spec := networkingv1.NetworkPolicySpec{
		PodSelector: metav1.LabelSelector{
			MatchLabels: selectorLabels,
		},
	}

	for _, opt := range opts {
		opt(&spec)
	}

	// Derive PolicyTypes from rules present
	if len(spec.Ingress) > 0 {
		spec.PolicyTypes = append(spec.PolicyTypes, networkingv1.PolicyTypeIngress)
	}

	if len(spec.Egress) > 0 {
		spec.PolicyTypes = append(spec.PolicyTypes, networkingv1.PolicyTypeEgress)
	}

	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      NetworkPolicyPrefix + name.Name,
			Namespace: name.Namespace,
			Labels:    labels,
		},
		Spec: spec,
	}
}

// WithIngressFromAny allows ingress traffic from any IP (0.0.0.0/0 and ::/0) on the specified TCP ports
func WithIngressFromAny(ports ...int32) NetworkPolicyOption {
	return func(spec *networkingv1.NetworkPolicySpec) {
		spec.Ingress = append(spec.Ingress, networkingv1.NetworkPolicyIngressRule{
			From: []networkingv1.NetworkPolicyPeer{
				{IPBlock: &networkingv1.IPBlock{CIDR: "0.0.0.0/0"}},
				{IPBlock: &networkingv1.IPBlock{CIDR: "::/0"}},
			},
			Ports: makeNetworkPolicyPorts(ports),
		})
	}
}

// WithIngressFromPods allows ingress traffic from pods matching the selector in the same namespace on the given TCP ports.
func WithIngressFromPods(selector map[string]string, ports ...int32) NetworkPolicyOption {
	return func(spec *networkingv1.NetworkPolicySpec) {
		spec.Ingress = append(spec.Ingress, networkingv1.NetworkPolicyIngressRule{
			From: []networkingv1.NetworkPolicyPeer{
				{
					PodSelector: &metav1.LabelSelector{
						MatchLabels: selector,
					},
				},
			},
			Ports: makeNetworkPolicyPorts(ports),
		})
	}
}

// WithIngressFromPodsInNamespace allows ingress traffic from pods matching the selector in a specific namespace on the given TCP ports.
func WithIngressFromPodsInNamespace(namespace string, selector map[string]string, ports ...int32) NetworkPolicyOption {
	return func(spec *networkingv1.NetworkPolicySpec) {
		spec.Ingress = append(spec.Ingress, networkingv1.NetworkPolicyIngressRule{
			From: []networkingv1.NetworkPolicyPeer{
				{
					PodSelector: &metav1.LabelSelector{
						MatchLabels: selector,
					},
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"kubernetes.io/metadata.name": namespace,
						},
					},
				},
			},
			Ports: makeNetworkPolicyPorts(ports),
		})
	}
}

// WithEgressToAny allows egress traffic to any IP (0.0.0.0/0 and ::/0)
func WithEgressToAny() NetworkPolicyOption {
	return func(spec *networkingv1.NetworkPolicySpec) {
		spec.Egress = append(spec.Egress, networkingv1.NetworkPolicyEgressRule{
			To: []networkingv1.NetworkPolicyPeer{
				{IPBlock: &networkingv1.IPBlock{CIDR: "0.0.0.0/0"}},
				{IPBlock: &networkingv1.IPBlock{CIDR: "::/0"}},
			},
		})
	}
}

// WithEgressToPods allows egress traffic to pods matching the selector in the same namespace on the given TCP ports.
func WithEgressToPods(selector map[string]string, ports ...int32) NetworkPolicyOption {
	return func(spec *networkingv1.NetworkPolicySpec) {
		spec.Egress = append(spec.Egress, networkingv1.NetworkPolicyEgressRule{
			To: []networkingv1.NetworkPolicyPeer{
				{
					PodSelector: &metav1.LabelSelector{
						MatchLabels: selector,
					},
				},
			},
			Ports: makeNetworkPolicyPorts(ports),
		})
	}
}

// WithEgressToPodsInNamespace allows egress traffic to pods matching the selector in a specific namespace on the given TCP ports.
func WithEgressToPodsInNamespace(namespace string, selector map[string]string, ports ...int32) NetworkPolicyOption {
	return func(spec *networkingv1.NetworkPolicySpec) {
		spec.Egress = append(spec.Egress, networkingv1.NetworkPolicyEgressRule{
			To: []networkingv1.NetworkPolicyPeer{
				{
					PodSelector: &metav1.LabelSelector{
						MatchLabels: selector,
					},
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"kubernetes.io/metadata.name": namespace,
						},
					},
				},
			},
			Ports: makeNetworkPolicyPorts(ports),
		})
	}
}

// WithIngressRule adds a raw NetworkPolicyIngressRule for advanced use cases
func WithIngressRule(rule networkingv1.NetworkPolicyIngressRule) NetworkPolicyOption {
	return func(spec *networkingv1.NetworkPolicySpec) {
		spec.Ingress = append(spec.Ingress, rule)
	}
}

// WithEgressRule adds a raw NetworkPolicyEgressRule for advanced use cases
func WithEgressRule(rule networkingv1.NetworkPolicyEgressRule) NetworkPolicyOption {
	return func(spec *networkingv1.NetworkPolicySpec) {
		spec.Egress = append(spec.Egress, rule)
	}
}

func makeNetworkPolicyPorts(ports []int32) []networkingv1.NetworkPolicyPort {
	var networkPolicyPorts []networkingv1.NetworkPolicyPort

	tcpProtocol := corev1.ProtocolTCP

	for idx := range ports {
		port := intstr.FromInt32(ports[idx])
		networkPolicyPorts = append(networkPolicyPorts, networkingv1.NetworkPolicyPort{
			Protocol: &tcpProtocol,
			Port:     &port,
		})
	}

	return networkPolicyPorts
}
