package common

import (
	"maps"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	SystemLogCollectorName       = "system-logs-collector"
	SystemLogAgentName           = "system-logs-agent"
	ClusterTrustBundleVolumeName = "custom-ca-bundle"
	ClusterTrustBundleFileName   = "ca-certificates.crt"
	ClusterTrustBundleVolumePath = "/etc/ssl/certs"
)

func MakeServiceAccount(name types.NamespacedName) *corev1.ServiceAccount {
	serviceAccount := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
		},
	}

	return &serviceAccount
}

func MakeClusterRoleBinding(name types.NamespacedName) *rbacv1.ClusterRoleBinding {
	clusterRoleBinding := rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
		},
		Subjects: []rbacv1.Subject{{Name: name.Name, Namespace: name.Namespace, Kind: "ServiceAccount"}},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     name.Name,
		},
	}

	return &clusterRoleBinding
}

func MakeNetworkPolicy(name types.NamespacedName, ingressAllowedPorts []int32, labels map[string]string, selectorLabels map[string]string) *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
			Labels:    labels,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress,
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							IPBlock: &networkingv1.IPBlock{CIDR: "0.0.0.0/0"},
						},
						{
							IPBlock: &networkingv1.IPBlock{CIDR: "::/0"},
						},
					},
					Ports: makeNetworkPolicyPorts(ingressAllowedPorts),
				},
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				{
					To: []networkingv1.NetworkPolicyPeer{
						{
							IPBlock: &networkingv1.IPBlock{CIDR: "0.0.0.0/0"},
						},
						{
							IPBlock: &networkingv1.IPBlock{CIDR: "::/0"},
						},
					},
				},
			},
		},
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

// ResourceMetadata holds labels and annotations for Kubernetes resources and their pods
type ResourceMetadata struct {
	ResourceLabels      map[string]string
	PodLabels           map[string]string
	ResourceAnnotations map[string]string
	PodAnnotations      map[string]string
}

// MakeResourceMetadata prepares labels and annotations for a Kubernetes resource and its pod template.
// It combines the provided baseLabels, extraPodLabels, baseAnnotations with global additional labels/annotations.
func MakeResourceMetadata(globals interface {
	AdditionalLabels() map[string]string
	AdditionalAnnotations() map[string]string
}, baseLabels, extraPodLabels, baseAnnotations map[string]string) ResourceMetadata {
	// Prepare annotations
	podAnnotations := make(map[string]string)
	resourceAnnotations := make(map[string]string)
	maps.Copy(resourceAnnotations, globals.AdditionalAnnotations())
	maps.Copy(podAnnotations, globals.AdditionalAnnotations())
	maps.Copy(podAnnotations, baseAnnotations)

	// Prepare pod labels
	defaultPodLabels := make(map[string]string)
	maps.Copy(defaultPodLabels, baseLabels)
	maps.Copy(defaultPodLabels, extraPodLabels)

	// Prepare resource and pod labels
	resourceLabels := make(map[string]string)
	podLabels := make(map[string]string)

	maps.Copy(resourceLabels, globals.AdditionalLabels())
	maps.Copy(podLabels, globals.AdditionalLabels())
	maps.Copy(resourceLabels, baseLabels)
	maps.Copy(podLabels, defaultPodLabels)

	return ResourceMetadata{
		ResourceLabels:      resourceLabels,
		PodLabels:           podLabels,
		ResourceAnnotations: resourceAnnotations,
		PodAnnotations:      podAnnotations,
	}
}
