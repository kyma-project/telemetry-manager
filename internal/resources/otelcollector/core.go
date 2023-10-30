package otelcollector

import (
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"strconv"
)

func defaultLabels(baseName string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name": baseName,
	}
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
					TargetPort: intstr.FromInt(ports.OTLPGRPC),
				},
				{
					Name:       "http-collector",
					Protocol:   corev1.ProtocolTCP,
					Port:       ports.OTLPHTTP,
					TargetPort: intstr.FromInt(ports.OTLPHTTP),
				},
			},
			Selector:        labels,
			Type:            corev1.ServiceTypeClusterIP,
			SessionAffinity: corev1.ServiceAffinityClientIP,
		},
	}
}

func makeMetricsService(config *Config) *corev1.Service {
	labels := defaultLabels(config.BaseName)

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.BaseName + "-metrics",
			Namespace: config.Namespace,
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
					TargetPort: intstr.FromInt(ports.Metrics),
				},
			},
			Selector: labels,
			Type:     corev1.ServiceTypeClusterIP,
		},
	}
}

func makeOpenCensusService(config *Config) *corev1.Service {
	labels := defaultLabels(config.BaseName)
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.BaseName + "-internal",
			Namespace: config.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "http-opencensus",
					Protocol:   corev1.ProtocolTCP,
					Port:       ports.OpenCensus,
					TargetPort: intstr.FromInt(ports.OpenCensus),
				},
			},
			Selector:        labels,
			Type:            corev1.ServiceTypeClusterIP,
			SessionAffinity: corev1.ServiceAffinityClientIP,
		},
	}
}

func makeNetworkPolicy(config *Config) *networkingv1.NetworkPolicy {
	labels := defaultLabels(config.BaseName)

	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.BaseName + "-pprof-deny-ingress",
			Namespace: config.Namespace,
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
		intstr.FromInt(ports.OTLPHTTP),
		intstr.FromInt(ports.OTLPGRPC),
		intstr.FromInt(ports.OpenCensus),
		intstr.FromInt(ports.Metrics),
		intstr.FromInt(ports.HealthCheck),
	}
}
