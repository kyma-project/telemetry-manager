package gateway

import (
	"maps"
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector/core"
)

type Config struct {
	BaseName   string
	Namespace  string
	Deployment DeploymentConfig
	Service    ServiceConfig
}

type DeploymentConfig struct {
	Image                string
	PriorityClassName    string
	BaseCPULimit         resource.Quantity
	DynamicCPULimit      resource.Quantity
	BaseMemoryLimit      resource.Quantity
	DynamicMemoryLimit   resource.Quantity
	BaseCPURequest       resource.Quantity
	DynamicCPURequest    resource.Quantity
	BaseMemoryRequest    resource.Quantity
	DynamicMemoryRequest resource.Quantity
}

type ServiceConfig struct {
	OTLPServiceName string
}

type Scaling struct {
	// Replicas specifies the desired number of gateway replicas.
	Replicas int32

	// ResourceRequirementsMultiplier is a coefficient affecting the CPU and memory resource limits for each replica.
	// This value is multiplied with a base resource requirement to calculate the actual CPU and memory limits.
	// A value of 1 will apply the base limits, values greater than 1 will proportionally increase those limits.
	ResourceRequirementsMultiplier int
}

func MakeClusterRole(name types.NamespacedName) *rbacv1.ClusterRole {
	clusterRole := rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"namespaces", "pods"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"apps"},
				Resources: []string{"replicasets"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	}
	return &clusterRole
}

func MakeSecret(config Config, secretData map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.BaseName,
			Namespace: config.Namespace,
			Labels:    core.MakeDefaultLabels(config.BaseName),
		},
		Data: secretData,
	}
}

func MakeDeployment(config Config, configHash string, scaling Scaling, envVarPodIP, envVarNodeName string) *appsv1.Deployment {
	selectorLabels := core.MakeDefaultLabels(config.BaseName)
	podLabels := maps.Clone(selectorLabels)
	podLabels["sidecar.istio.io/inject"] = "false"

	annotations := core.MakeCommonPodAnnotations(configHash)

	resources := makeResourceRequirements(config, scaling)
	affinity := makePodAffinity(selectorLabels)
	podSpec := core.MakePodSpec(config.BaseName, config.Deployment.Image,
		core.WithPriorityClass(config.Deployment.PriorityClassName),
		core.WithResources(resources),
		core.WithAffinity(affinity),
		core.WithEnvVarFromSource(envVarPodIP, core.FieldPathPodIP),
		core.WithEnvVarFromSource(envVarNodeName, core.FieldPathNodeName),
	)

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.BaseName,
			Namespace: config.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32(scaling.Replicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      podLabels,
					Annotations: annotations,
				},
				Spec: podSpec,
			},
		},
	}
}

// makeResourceRequirements returns the resource requirements for the opentelemetry-collector. We calculate the resources based on the initial base value and a dynamic part per pipeline.
func makeResourceRequirements(config Config, scaling Scaling) corev1.ResourceRequirements {
	memoryRequest := config.Deployment.BaseMemoryRequest.DeepCopy()
	memoryLimit := config.Deployment.BaseMemoryLimit.DeepCopy()
	cpuRequest := config.Deployment.BaseCPURequest.DeepCopy()
	cpuLimit := config.Deployment.BaseCPULimit.DeepCopy()

	for i := 0; i < scaling.ResourceRequirementsMultiplier; i++ {
		memoryRequest.Add(config.Deployment.DynamicMemoryRequest)
		memoryLimit.Add(config.Deployment.DynamicMemoryLimit)
		cpuRequest.Add(config.Deployment.DynamicCPURequest)
		cpuLimit.Add(config.Deployment.DynamicCPULimit)
	}

	resources := corev1.ResourceRequirements{
		Requests: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    cpuRequest,
			corev1.ResourceMemory: memoryRequest,
		},
		Limits: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    cpuLimit,
			corev1.ResourceMemory: memoryLimit,
		},
	}

	return resources
}

func makePodAffinity(labels map[string]string) corev1.Affinity {
	return corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
				{
					Weight: 100,
					PodAffinityTerm: corev1.PodAffinityTerm{
						TopologyKey: "kubernetes.io/hostname",
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: labels,
						},
					},
				},
				{
					Weight: 100,
					PodAffinityTerm: corev1.PodAffinityTerm{
						TopologyKey: "topology.kubernetes.io/zone",
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: labels,
						},
					},
				},
			},
		},
	}
}

func MakeOTLPService(config Config) *corev1.Service {
	labels := core.MakeDefaultLabels(config.BaseName)
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.Service.OTLPServiceName,
			Namespace: config.Namespace,
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

func MakeMetricsService(config Config) *corev1.Service {
	labels := core.MakeDefaultLabels(config.BaseName)

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

func MakeOpenCensusService(config Config) *corev1.Service {
	labels := core.MakeDefaultLabels(config.BaseName)
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

func MakeNetworkPolicy(config Config, ports []intstr.IntOrString) *networkingv1.NetworkPolicy {
	labels := core.MakeDefaultLabels(config.BaseName)
	networkPolicyPorts := makeNetworkPolicyPorts(ports)

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
					Ports: networkPolicyPorts,
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
