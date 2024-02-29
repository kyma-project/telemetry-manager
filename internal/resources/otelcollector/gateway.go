package otelcollector

import (
	"context"
	"fmt"
	"maps"

	istiosecurityv1beta "istio.io/api/security/v1beta1"
	istiotypev1beta1 "istio.io/api/type/v1beta1"
	istiosecurityclientv1beta "istio.io/client-go/pkg/apis/security/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/configchecksum"
	"github.com/kyma-project/telemetry-manager/internal/k8sutils"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
)

func ApplyGatewayResources(ctx context.Context, c client.Client, cfg *GatewayConfig) error {
	name := types.NamespacedName{Namespace: cfg.Namespace, Name: cfg.BaseName}

	if err := applyCommonResources(ctx, c, name, makeGatewayClusterRole(name), cfg.allowedPorts); err != nil {
		return fmt.Errorf("failed to create common resource: %w", err)
	}

	secret := makeSecret(name, cfg.CollectorEnvVars)
	if err := k8sutils.CreateOrUpdateSecret(ctx, c, secret); err != nil {
		return fmt.Errorf("failed to create env secret: %w", err)
	}

	configMap := makeConfigMap(name, cfg.CollectorConfig)
	if err := k8sutils.CreateOrUpdateConfigMap(ctx, c, configMap); err != nil {
		return fmt.Errorf("failed to create configmap: %w", err)
	}

	configChecksum := configchecksum.Calculate([]corev1.ConfigMap{*configMap}, []corev1.Secret{*secret})
	if err := k8sutils.CreateOrUpdateDeployment(ctx, c, makeGatewayDeployment(cfg, configChecksum, cfg.Istio)); err != nil {
		return fmt.Errorf("failed to create deployment: %w", err)
	}

	if err := k8sutils.CreateOrUpdateService(ctx, c, makeOTLPService(cfg)); err != nil {
		return fmt.Errorf("failed to create otlp service: %w", err)
	}

	if cfg.CanReceiveOpenCensus {
		if err := k8sutils.CreateOrUpdateService(ctx, c, makeOpenCensusService(name)); err != nil {
			return fmt.Errorf("failed to create open census service: %w", err)
		}
	}

	if cfg.Istio.Enabled {
		if err := k8sutils.CreateOrUpdatePeerAuthentication(ctx, c, makePeerAuthentication(cfg)); err != nil {
			return fmt.Errorf("failed to create peerauthentication: %w", err)
		}
	}

	return nil
}

func makeGatewayClusterRole(name types.NamespacedName) *rbacv1.ClusterRole {
	clusterRole := rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
			Labels:    defaultLabels(name.Name),
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

func makeGatewayDeployment(cfg *GatewayConfig, configChecksum string, istioConfig IstioConfig) *appsv1.Deployment {
	selectorLabels := defaultLabels(cfg.BaseName)
	podLabels := maps.Clone(selectorLabels)
	podLabels["sidecar.istio.io/inject"] = fmt.Sprintf("%t", istioConfig.Enabled)

	annotations := map[string]string{"checksum/config": configChecksum}
	if istioConfig.Enabled {
		annotations["traffic.sidecar.istio.io/excludeInboundPorts"] = istioConfig.ExcludePorts
		// When a workload is outside the istio mesh and communicates with pod in service mesh, the envoy proxy does not
		// preserve the source IP and destination IP. To preserve source/destination IP we need TPROXY interception mode.
		// More info: https://istio.io/latest/docs/reference/config/istio.mesh.v1alpha1/#ProxyConfig-InboundInterceptionMode
		annotations["sidecar.istio.io/interceptionMode"] = "TPROXY"
	}
	resources := makeGatewayResourceRequirements(cfg)
	affinity := makePodAffinity(selectorLabels)
	podSpec := makePodSpec(cfg.BaseName, cfg.Deployment.Image,
		commonresources.WithPriorityClass(cfg.Deployment.PriorityClassName),
		commonresources.WithResources(resources),
		withAffinity(affinity),
		withEnvVarFromSource(config.EnvVarCurrentPodIP, fieldPathPodIP),
		withEnvVarFromSource(config.EnvVarCurrentNodeName, fieldPathNodeName),
	)

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cfg.BaseName,
			Namespace: cfg.Namespace,
			Labels:    selectorLabels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(cfg.Scaling.Replicas),
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

func makeGatewayResourceRequirements(cfg *GatewayConfig) corev1.ResourceRequirements {
	memoryRequest := cfg.Deployment.BaseMemoryRequest.DeepCopy()
	memoryLimit := cfg.Deployment.BaseMemoryLimit.DeepCopy()
	cpuRequest := cfg.Deployment.BaseCPURequest.DeepCopy()
	cpuLimit := cfg.Deployment.BaseCPULimit.DeepCopy()

	for i := 0; i < cfg.Scaling.ResourceRequirementsMultiplier; i++ {
		memoryRequest.Add(cfg.Deployment.DynamicMemoryRequest)
		memoryLimit.Add(cfg.Deployment.DynamicMemoryLimit)
		cpuRequest.Add(cfg.Deployment.DynamicCPURequest)
		cpuLimit.Add(cfg.Deployment.DynamicCPULimit)
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
			Selector: labels,
			Type:     corev1.ServiceTypeClusterIP,
		},
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
			Selector: labels,
			Type:     corev1.ServiceTypeClusterIP,
		},
	}
}

func makePeerAuthentication(cfg *GatewayConfig) *istiosecurityclientv1beta.PeerAuthentication {
	selectorLabels := defaultLabels(cfg.BaseName)

	return &istiosecurityclientv1beta.PeerAuthentication{
		ObjectMeta: metav1.ObjectMeta{Name: cfg.BaseName, Namespace: cfg.Namespace, Labels: selectorLabels},
		Spec: istiosecurityv1beta.PeerAuthentication{
			Selector: &istiotypev1beta1.WorkloadSelector{MatchLabels: defaultLabels(cfg.BaseName)},
			Mtls:     &istiosecurityv1beta.PeerAuthentication_MutualTLS{Mode: istiosecurityv1beta.PeerAuthentication_MutualTLS_PERMISSIVE},
		},
	}
}
