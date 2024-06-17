package otelcollector

import (
	"context"
	"fmt"
	"maps"
	"strings"

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

type GatewayResourcesHandler struct {
	Config GatewayConfig
}

type GatewayApplyOptions struct {
	AllowedPorts        []int32
	CollectorConfigYAML string
	CollectorEnvVars    map[string][]byte
	IstioEnabled        bool
	IstioExcludePorts   []int32

	// Replicas specifies the number of gateway replicas.
	Replicas int32

	// ResourceRequirementsMultiplier is a coefficient affecting the CPU and memory resource limits for each replica.
	// This value is multiplied with a base resource requirement to calculate the actual CPU and memory limits.
	// A value of 1 applies the base limits; values greater than 1 increase those limits proportionally.
	ResourceRequirementsMultiplier int
}

func (grh *GatewayResourcesHandler) ApplyResources(ctx context.Context, c client.Client, opts GatewayApplyOptions) error {
	name := types.NamespacedName{Namespace: grh.Config.Namespace, Name: grh.Config.BaseName}

	if err := applyCommonResources(ctx, c, name, grh.makeGatewayClusterRole(name), opts.AllowedPorts); err != nil {
		return fmt.Errorf("failed to create common resource: %w", err)
	}

	secret := makeSecret(name, opts.CollectorEnvVars)
	if err := k8sutils.CreateOrUpdateSecret(ctx, c, secret); err != nil {
		return fmt.Errorf("failed to create env secret: %w", err)
	}

	configMap := makeConfigMap(name, opts.CollectorConfigYAML)
	if err := k8sutils.CreateOrUpdateConfigMap(ctx, c, configMap); err != nil {
		return fmt.Errorf("failed to create configmap: %w", err)
	}

	configChecksum := configchecksum.Calculate([]corev1.ConfigMap{*configMap}, []corev1.Secret{*secret})
	if err := k8sutils.CreateOrUpdateDeployment(ctx, c, grh.makeGatewayDeployment(configChecksum, opts)); err != nil {
		return fmt.Errorf("failed to create deployment: %w", err)
	}

	if err := k8sutils.CreateOrUpdateService(ctx, c, grh.makeOTLPService()); err != nil {
		return fmt.Errorf("failed to create otlp service: %w", err)
	}

	if opts.IstioEnabled {
		if err := k8sutils.CreateOrUpdatePeerAuthentication(ctx, c, grh.makePeerAuthentication()); err != nil {
			return fmt.Errorf("failed to create peerauthentication: %w", err)
		}
	}

	return nil
}

func (grh *GatewayResourcesHandler) DeleteResources(ctx context.Context, c client.Client, isIstioActive bool) error {
	name := types.NamespacedName{Name: grh.Config.BaseName, Namespace: grh.Config.Namespace}
	if err := deleteCommonResources(ctx, c, name); err != nil {
		return fmt.Errorf("failed to delete common resource: %w", err)
	}

	objectMeta := metav1.ObjectMeta{
		Name:      grh.Config.BaseName,
		Namespace: grh.Config.Namespace,
	}

	secret := corev1.Secret{ObjectMeta: objectMeta}
	if err := k8sutils.DeleteObject(ctx, c, &secret); err != nil {
		return fmt.Errorf("failed to delete env secret: %w", err)
	}

	configMap := corev1.ConfigMap{ObjectMeta: objectMeta}
	if err := k8sutils.DeleteObject(ctx, c, &configMap); err != nil {
		return fmt.Errorf("failed to delete configmap: %w", err)
	}

	deployment := appsv1.Deployment{ObjectMeta: objectMeta}
	if err := k8sutils.DeleteObject(ctx, c, &deployment); err != nil {
		return fmt.Errorf("failed to delete deployment: %w", err)
	}

	OTLPService := corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: grh.Config.OTLPServiceName, Namespace: grh.Config.Namespace}}
	if err := k8sutils.DeleteObject(ctx, c, &OTLPService); err != nil {
		return fmt.Errorf("failed to delete otlp service: %w", err)
	}

	if isIstioActive {
		peerAuthentication := istiosecurityclientv1beta.PeerAuthentication{ObjectMeta: objectMeta}
		if err := k8sutils.DeleteObject(ctx, c, &peerAuthentication); err != nil {
			return fmt.Errorf("failed to delete peerauthentication: %w", err)
		}
	}

	return nil
}

func (grh *GatewayResourcesHandler) makeGatewayClusterRole(name types.NamespacedName) *rbacv1.ClusterRole {
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

func (grh *GatewayResourcesHandler) makeGatewayDeployment(configChecksum string, opts GatewayApplyOptions) *appsv1.Deployment {
	selectorLabels := defaultLabels(grh.Config.BaseName)
	podLabels := maps.Clone(selectorLabels)
	podLabels["sidecar.istio.io/inject"] = fmt.Sprintf("%t", opts.IstioEnabled)

	annotations := map[string]string{"checksum/config": configChecksum}
	if opts.IstioEnabled {
		var excludeInboundPorts []string
		for _, p := range opts.IstioExcludePorts {
			excludeInboundPorts = append(excludeInboundPorts, fmt.Sprintf("%d", p))
		}
		annotations["traffic.sidecar.istio.io/excludeInboundPorts"] = strings.Join(excludeInboundPorts, ", ")
		// When a workload is outside the istio mesh and communicates with pod in service mesh, the envoy proxy does not
		// preserve the source IP and destination IP. To preserve source/destination IP we need TPROXY interception mode.
		// More info: https://istio.io/latest/docs/reference/config/istio.mesh.v1alpha1/#ProxyConfig-InboundInterceptionMode
		annotations["sidecar.istio.io/interceptionMode"] = "TPROXY"
	}
	resources := grh.makeGatewayResourceRequirements(opts)
	affinity := makePodAffinity(selectorLabels)

	deploymentConfig := grh.Config.Deployment
	podSpec := makePodSpec(
		grh.Config.BaseName,
		deploymentConfig.Image,
		commonresources.WithPriorityClass(deploymentConfig.PriorityClassName),
		commonresources.WithResources(resources),
		withAffinity(affinity),
		withEnvVarFromSource(config.EnvVarCurrentPodIP, fieldPathPodIP),
		withEnvVarFromSource(config.EnvVarCurrentNodeName, fieldPathNodeName),
		commonresources.WithGoMemLimitEnvVar(resources.Limits[corev1.ResourceMemory]),
	)

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      grh.Config.BaseName,
			Namespace: grh.Config.Namespace,
			Labels:    selectorLabels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(opts.Replicas),
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

func (grh *GatewayResourcesHandler) makeGatewayResourceRequirements(opts GatewayApplyOptions) corev1.ResourceRequirements {
	deploymentConfig := grh.Config.Deployment

	memoryRequest := deploymentConfig.BaseMemoryRequest.DeepCopy()
	memoryLimit := deploymentConfig.BaseMemoryLimit.DeepCopy()
	cpuRequest := deploymentConfig.BaseCPURequest.DeepCopy()
	cpuLimit := deploymentConfig.BaseCPULimit.DeepCopy()

	for i := 0; i < opts.ResourceRequirementsMultiplier; i++ {
		memoryRequest.Add(deploymentConfig.DynamicMemoryRequest)
		memoryLimit.Add(deploymentConfig.DynamicMemoryLimit)
		cpuRequest.Add(deploymentConfig.DynamicCPURequest)
		cpuLimit.Add(deploymentConfig.DynamicCPULimit)
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

func (grh *GatewayResourcesHandler) makeOTLPService() *corev1.Service {
	labels := defaultLabels(grh.Config.BaseName)

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      grh.Config.OTLPServiceName,
			Namespace: grh.Config.Namespace,
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

func (grh *GatewayResourcesHandler) makePeerAuthentication() *istiosecurityclientv1beta.PeerAuthentication {
	labels := defaultLabels(grh.Config.BaseName)

	return &istiosecurityclientv1beta.PeerAuthentication{
		ObjectMeta: metav1.ObjectMeta{
			Name:      grh.Config.BaseName,
			Namespace: grh.Config.Namespace,
			Labels:    labels,
		},
		Spec: istiosecurityv1beta.PeerAuthentication{
			Selector: &istiotypev1beta1.WorkloadSelector{MatchLabels: labels},
			Mtls:     &istiosecurityv1beta.PeerAuthentication_MutualTLS{Mode: istiosecurityv1beta.PeerAuthentication_MutualTLS_PERMISSIVE},
		},
	}
}
