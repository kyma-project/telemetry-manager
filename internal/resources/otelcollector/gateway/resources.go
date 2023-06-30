package gateway

import (
	collectorconfig "github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector/core"
	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
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

const (
	configHashAnnotationKey = "checksum/config"
)

var (
	configMapKey          = "relay.conf"
	defaultPodAnnotations = map[string]string{
		"sidecar.istio.io/inject": "false",
	}
	replicas = int32(2)
)

func MakeConfigMap(config Config, collectorConfig collectorconfig.Config) *corev1.ConfigMap {
	bytes, _ := yaml.Marshal(collectorConfig)
	confYAML := string(bytes)

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.BaseName,
			Namespace: config.Namespace,
			Labels:    core.MakeDefaultLabels(config.BaseName),
		},
		Data: map[string]string{
			configMapKey: confYAML,
		},
	}
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

func MakeDeployment(config Config, configHash string, pipelineCount int) *appsv1.Deployment {
	labels := core.MakeDefaultLabels(config.BaseName)
	annotations := makePodAnnotations(configHash)
	resources := makeResourceRequirements(config, pipelineCount)
	podSpec := core.MakePodSpec(config.BaseName, config.Deployment.Image, config.Deployment.PriorityClassName, resources)

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.BaseName,
			Namespace: config.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: annotations,
				},
				Spec: podSpec,
			},
		},
	}
}

func makePodAnnotations(configHash string) map[string]string {
	annotations := map[string]string{
		configHashAnnotationKey: configHash,
	}
	for k, v := range defaultPodAnnotations {
		annotations[k] = v
	}
	return annotations
}

// makeResourceRequirements returns the resource requirements for the opentelemetry-collector. We calculate the resources based on a initial base value and a dynamic part per pipeline.
func makeResourceRequirements(config Config, pipelineCount int) corev1.ResourceRequirements {
	memoryRequest := config.Deployment.BaseMemoryRequest.DeepCopy()
	memoryLimit := config.Deployment.BaseMemoryLimit.DeepCopy()
	cpuRequest := config.Deployment.BaseCPURequest.DeepCopy()
	cpuLimit := config.Deployment.BaseCPULimit.DeepCopy()

	for i := 0; i < pipelineCount; i++ {
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
					Port:       4317,
					TargetPort: intstr.FromInt(4317),
				},
				{
					Name:       "http-collector",
					Protocol:   corev1.ProtocolTCP,
					Port:       4318,
					TargetPort: intstr.FromInt(4318),
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
				"prometheus.io/port":   "8888",
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "http-metrics",
					Protocol:   corev1.ProtocolTCP,
					Port:       8888,
					TargetPort: intstr.FromInt(8888),
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
					Port:       55678,
					TargetPort: intstr.FromInt(55678),
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
