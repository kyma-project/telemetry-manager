package common

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type NetworkPolicyOption func(*networkingv1.NetworkPolicy)

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
	networkPolicy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      NetworkPolicyPrefix + name.Name,
			Namespace: name.Namespace,
			Labels:    labels,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
		},
	}

	for _, opt := range opts {
		opt(networkPolicy)
	}

	// Derive PolicyTypes from rules present
	if len(networkPolicy.Spec.Ingress) > 0 {
		networkPolicy.Spec.PolicyTypes = append(networkPolicy.Spec.PolicyTypes, networkingv1.PolicyTypeIngress)
	}

	if len(networkPolicy.Spec.Egress) > 0 {
		networkPolicy.Spec.PolicyTypes = append(networkPolicy.Spec.PolicyTypes, networkingv1.PolicyTypeEgress)
	}

	return networkPolicy
}

func WithNameSuffix(suffix string) func(spec *networkingv1.NetworkPolicy) {
	return func(np *networkingv1.NetworkPolicy) {
		np.Name = np.Name + "-" + suffix
	}
}

// WithIngressFromAny allows ingress traffic from any IP (0.0.0.0/0 and ::/0) on the specified TCP ports
func WithIngressFromAny(ports []int32) NetworkPolicyOption {
	return func(netpol *networkingv1.NetworkPolicy) {
		netpol.Spec.Ingress = append(netpol.Spec.Ingress, networkingv1.NetworkPolicyIngressRule{
			From: []networkingv1.NetworkPolicyPeer{
				{IPBlock: &networkingv1.IPBlock{CIDR: "0.0.0.0/0"}},
				{IPBlock: &networkingv1.IPBlock{CIDR: "::/0"}},
			},
			Ports: makeNetworkPolicyPorts(ports),
		})
	}
}

// WithIngressFromPods allows ingress traffic from pods matching the selector in the same namespace on the given TCP ports.
func WithIngressFromPods(selector map[string]string, ports []int32) NetworkPolicyOption {
	return func(netpol *networkingv1.NetworkPolicy) {
		netpol.Spec.Ingress = append(netpol.Spec.Ingress, networkingv1.NetworkPolicyIngressRule{
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

// WithIngressFromPodsInAllNamespaces allows ingress traffic from pods matching the selector in any namespace on the given TCP ports.
func WithIngressFromPodsInAllNamespaces(selector map[string]string, ports []int32) NetworkPolicyOption {
	return func(netpol *networkingv1.NetworkPolicy) {
		netpol.Spec.Ingress = append(netpol.Spec.Ingress, networkingv1.NetworkPolicyIngressRule{
			From: []networkingv1.NetworkPolicyPeer{
				{
					PodSelector: &metav1.LabelSelector{
						MatchLabels: selector,
					},
					NamespaceSelector: &metav1.LabelSelector{},
				},
			},
			Ports: makeNetworkPolicyPorts(ports),
		})
	}
}

// WithIngressFromPodsInNamespace allows ingress traffic from pods matching the selector in a specific namespace on the given TCP ports.
func WithIngressFromPodsInNamespace(namespace string, selector map[string]string, ports []int32) NetworkPolicyOption {
	return func(netpol *networkingv1.NetworkPolicy) {
		netpol.Spec.Ingress = append(netpol.Spec.Ingress, networkingv1.NetworkPolicyIngressRule{
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
	return func(netpol *networkingv1.NetworkPolicy) {
		netpol.Spec.Egress = append(netpol.Spec.Egress, networkingv1.NetworkPolicyEgressRule{
			To: []networkingv1.NetworkPolicyPeer{
				{IPBlock: &networkingv1.IPBlock{CIDR: "0.0.0.0/0"}},
				{IPBlock: &networkingv1.IPBlock{CIDR: "::/0"}},
			},
		})
	}
}

// WithEgressToPods allows egress traffic to pods matching the selector in the same namespace on the given TCP ports.
func WithEgressToPods(selector map[string]string, ports []int32) NetworkPolicyOption {
	return func(netpol *networkingv1.NetworkPolicy) {
		netpol.Spec.Egress = append(netpol.Spec.Egress, networkingv1.NetworkPolicyEgressRule{
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

// WithEgressToPodsInAllNamespaces allows egress traffic to pods matching the selector in any namespace on the given TCP ports.
func WithEgressToPodsInAllNamespaces(selector map[string]string, ports []int32) NetworkPolicyOption {
	return func(netpol *networkingv1.NetworkPolicy) {
		netpol.Spec.Egress = append(netpol.Spec.Egress, networkingv1.NetworkPolicyEgressRule{
			To: []networkingv1.NetworkPolicyPeer{
				{
					PodSelector: &metav1.LabelSelector{
						MatchLabels: selector,
					},
					NamespaceSelector: &metav1.LabelSelector{},
				},
			},
			Ports: makeNetworkPolicyPorts(ports),
		})
	}
}

// WithEgressToPodsInNamespace allows egress traffic to pods matching the selector in a specific namespace on the given TCP ports.
func WithEgressToPodsInNamespace(namespace string, selector map[string]string, ports ...int32) NetworkPolicyOption {
	return func(netpol *networkingv1.NetworkPolicy) {
		netpol.Spec.Egress = append(netpol.Spec.Egress, networkingv1.NetworkPolicyEgressRule{
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
	return func(netpol *networkingv1.NetworkPolicy) {
		netpol.Spec.Ingress = append(netpol.Spec.Ingress, rule)
	}
}

// WithEgressRule adds a raw NetworkPolicyEgressRule for advanced use cases
func WithEgressRule(rule networkingv1.NetworkPolicyEgressRule) NetworkPolicyOption {
	return func(netpol *networkingv1.NetworkPolicy) {
		netpol.Spec.Egress = append(netpol.Spec.Egress, rule)
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

// TODO: Remove after rollout 1.59.0

func CleanupOldNetworkPolicy(ctx context.Context, c client.Client, name types.NamespacedName) error {
	oldNetworkPolicy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
		},
	}

	if err := c.Delete(ctx, oldNetworkPolicy); err != nil {
		if apierrors.IsNotFound(err) {
			// Already deleted, ignore
			return nil
		}

		return err
	}

	return nil
}
