package otelcollector

import (
	"strconv"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	"context"
	"fmt"
	"github.com/kyma-project/telemetry-manager/internal/kubernetes"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// applyCommonResources applies resources to gateway and agent deployment node
func applyCommonResources(ctx context.Context, c client.Client, name types.NamespacedName) error {
	if err := kubernetes.CreateOrUpdateServiceAccount(ctx, c, makeServiceAccount(name)); err != nil {
		return fmt.Errorf("failed to create service account: %w", err)
	}

	if err := kubernetes.CreateOrUpdateClusterRoleBinding(ctx, c, makeClusterRoleBinding(name)); err != nil {
		return fmt.Errorf("failed to create cluster role binding: %w", err)
	}

	if err := kubernetes.CreateOrUpdateService(ctx, c, makeMetricsService(name)); err != nil {
		return fmt.Errorf("failed to create metrics service: %w", err)
	}

	if err := kubernetes.CreateOrUpdateNetworkPolicy(ctx, c, makeDenyPprofNetworkPolicy(name)); err != nil {
		return fmt.Errorf("failed to create deny pprof network policy: %w", err)
	}

	return nil
}

func defaultLabels(baseName string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name": baseName,
	}
}

func makeServiceAccount(name types.NamespacedName) *corev1.ServiceAccount {
	serviceAccount := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
			Labels:    defaultLabels(name.Name),
		},
	}
	return &serviceAccount
}

func makeClusterRoleBinding(name types.NamespacedName) *rbacv1.ClusterRoleBinding {
	clusterRoleBinding := rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
			Labels:    defaultLabels(name.Name),
		},
		Subjects: []rbacv1.Subject{{Name: name.Name, Namespace: name.Namespace, Kind: rbacv1.ServiceAccountKind}},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     name.Name,
		},
	}
	return &clusterRoleBinding
}

func makeConfigMap(name types.NamespacedName, collectorConfig string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
			Labels:    defaultLabels(name.Name),
		},
		Data: map[string]string{
			configMapKey: collectorConfig,
		},
	}
}

func makeSecret(name types.NamespacedName, secretData map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
			Labels:    defaultLabels(name.Name),
		},
		Data: secretData,
	}
}

func makeOTLPService(cfg *GatewayConfig) *corev1.Service {
	labels := defaultLabels(cfg.BaseName)
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cfg.OTLPServiceName,
			Namespace: cfg.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "grpc-collector",
					Protocol:   corev1.ProtocolTCP,
					Port:       ports.OTLPGRPC,
					TargetPort: intstr.FromInt32(ports.OTLPGRPC),
				},
				{
					Name:       "http-collector",
					Protocol:   corev1.ProtocolTCP,
					Port:       ports.OTLPHTTP,
					TargetPort: intstr.FromInt32(ports.OTLPHTTP),
				},
			},
			Selector:        labels,
			Type:            corev1.ServiceTypeClusterIP,
			SessionAffinity: corev1.ServiceAffinityClientIP,
		},
	}
}

func makeMetricsService(name types.NamespacedName) *corev1.Service {
	labels := defaultLabels(name.Name)

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name + "-metrics",
			Namespace: name.Namespace,
			Labels:    labels,
			Annotations: map[string]string{
				"prometheus.io/scrape": "true",
				"prometheus.io/port":   strconv.Itoa(ports.Metrics),
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "http-metrics",
					Protocol:   corev1.ProtocolTCP,
					Port:       ports.Metrics,
					TargetPort: intstr.FromInt32(ports.Metrics),
				},
			},
			Selector: labels,
			Type:     corev1.ServiceTypeClusterIP,
		},
	}
}

func makeOpenCensusService(name types.NamespacedName) *corev1.Service {
	labels := defaultLabels(name.Name)
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name + "-internal",
			Namespace: name.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "http-opencensus",
					Protocol:   corev1.ProtocolTCP,
					Port:       ports.OpenCensus,
					TargetPort: intstr.FromInt32(ports.OpenCensus),
				},
			},
			Selector:        labels,
			Type:            corev1.ServiceTypeClusterIP,
			SessionAffinity: corev1.ServiceAffinityClientIP,
		},
	}
}

func makeDenyPprofNetworkPolicy(name types.NamespacedName) *networkingv1.NetworkPolicy {
	labels := defaultLabels(name.Name)

	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name + "-pprof-deny-ingress",
			Namespace: name.Namespace,
			Labels:    labels,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: labels,
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							IPBlock: &networkingv1.IPBlock{CIDR: "0.0.0.0/0"},
						},
					},
					Ports: makeNetworkPolicyPorts(collectorPorts()),
				},
			},
		},
	}
}

func makeNetworkPolicyPorts(ports []intstr.IntOrString) []networkingv1.NetworkPolicyPort {
	var networkPolicyPorts []networkingv1.NetworkPolicyPort

	tcpProtocol := corev1.ProtocolTCP

	for idx := range ports {
		networkPolicyPorts = append(networkPolicyPorts, networkingv1.NetworkPolicyPort{
			Protocol: &tcpProtocol,
			Port:     &ports[idx],
		})
	}

	return networkPolicyPorts
}

func collectorPorts() []intstr.IntOrString {
	return []intstr.IntOrString{
		intstr.FromInt32(ports.OTLPHTTP),
		intstr.FromInt32(ports.OTLPGRPC),
		intstr.FromInt32(ports.OpenCensus),
		intstr.FromInt32(ports.Metrics),
		intstr.FromInt32(ports.HealthCheck),
	}
}
