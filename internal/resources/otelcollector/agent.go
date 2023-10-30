package otelcollector

import (
	"context"
	"fmt"
	"maps"
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/configchecksum"
	"github.com/kyma-project/telemetry-manager/internal/kubernetes"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	configmetricagent "github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric/agent"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
)

const istioCertVolumeName = "istio-certs"

func ApplyAgentResources(ctx context.Context, c client.Client, cfg *AgentConfig) error {
	if err := applyAgentResources(ctx, c, cfg); err != nil {
		return fmt.Errorf("failed to apply common otel collector resource: %w", err)
	}

	return nil
}

func applyAgentResources(ctx context.Context, c client.Client, cfg *AgentConfig) error {
	name := types.NamespacedName{Namespace: cfg.Namespace, Name: cfg.BaseName}

	serviceAccount := commonresources.MakeServiceAccount(name)
	if err := kubernetes.CreateOrUpdateServiceAccount(ctx, c, serviceAccount); err != nil {
		return fmt.Errorf("failed to create otel collector service account: %w", err)
	}

	clusterRole := makeAgentClusterRole(name)
	if err := kubernetes.CreateOrUpdateClusterRole(ctx, c, clusterRole); err != nil {
		return fmt.Errorf("failed to create otel collector cluster role: %w", err)
	}

	clusterRoleBinding := commonresources.MakeClusterRoleBinding(name)
	if err := kubernetes.CreateOrUpdateClusterRoleBinding(ctx, c, clusterRoleBinding); err != nil {
		return fmt.Errorf("failed to create otel collector cluster role Binding: %w", err)
	}

	configMap := makeConfigMap(name, cfg.CollectorConfig)
	if err := kubernetes.CreateOrUpdateConfigMap(ctx, c, configMap); err != nil {
		return fmt.Errorf("failed to create configmap: %w", err)
	}

	configChecksum := configchecksum.Calculate([]corev1.ConfigMap{*configMap}, []corev1.Secret{})
	daemonSet := makeAgentDaemonSet(cfg, configChecksum)
	if err := kubernetes.CreateOrUpdateDaemonSet(ctx, c, daemonSet); err != nil {
		return fmt.Errorf("failed to create deployment: %w", err)
	}

	metricsService := makeMetricsService(&cfg.Config)
	if err := kubernetes.CreateOrUpdateService(ctx, c, metricsService); err != nil {
		return fmt.Errorf("failed to create otel collector metrics service: %w", err)
	}

	networkPolicy := makeNetworkPolicy(&cfg.Config)
	if err := kubernetes.CreateOrUpdateNetworkPolicy(ctx, c, networkPolicy); err != nil {
		return fmt.Errorf("failed to create otel collector network policy: %w", err)
	}

	return nil
}

func makeAgentClusterRole(name types.NamespacedName) *rbacv1.ClusterRole {
	clusterRole := rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"nodes", "nodes/metrics", "nodes/stats", "services", "endpoints", "pods"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				NonResourceURLs: []string{"/metrics", "/metrics/cadvisor"},
				Verbs:           []string{"get"},
			},
		},
	}
	return &clusterRole
}

func makeAgentDaemonSet(cfg *AgentConfig, configChecksum string) *appsv1.DaemonSet {
	selectorLabels := defaultLabels(cfg.BaseName)
	podLabels := maps.Clone(selectorLabels)
	podLabels["sidecar.istio.io/inject"] = "true"

	annotations := makeCommonPodAnnotations(configChecksum)
	maps.Copy(annotations, makeIstioTLSPodAnnotations(configmetricagent.IstioCertPath))

	resources := makeAgentResourceRequirements(cfg)
	podSpec := makePodSpec(cfg.BaseName, cfg.DaemonSet.Image,
		withPriorityClass(cfg.DaemonSet.PriorityClassName),
		withResources(resources),
		withEnvVarFromSource(config.EnvVarCurrentPodIP, fieldPathPodIP),
		withEnvVarFromSource(config.EnvVarCurrentNodeName, fieldPathNodeName),
		withVolume(corev1.Volume{Name: istioCertVolumeName, VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		}}),
		withVolumeMount(corev1.VolumeMount{
			Name:      istioCertVolumeName,
			MountPath: configmetricagent.IstioCertPath,
			ReadOnly:  true,
		}),
	)

	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cfg.BaseName,
			Namespace: cfg.Namespace,
		},
		Spec: appsv1.DaemonSetSpec{
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

func makeAgentResourceRequirements(cfg *AgentConfig) corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Limits: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    cfg.DaemonSet.CPULimit,
			corev1.ResourceMemory: cfg.DaemonSet.MemoryLimit,
		},
		Requests: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    cfg.DaemonSet.CPURequest,
			corev1.ResourceMemory: cfg.DaemonSet.MemoryRequest,
		},
	}
}

func makeIstioTLSPodAnnotations(istioCertPath string) map[string]string {
	return map[string]string{
		"proxy.istio.io/config": fmt.Sprintf(`# configure an env variable OUTPUT_CERTS to write certificates to the given folder
proxyMetadata:
  OUTPUT_CERTS: %s
`, istioCertPath),
		"sidecar.istio.io/userVolumeMount":              fmt.Sprintf(`[{"name": "%s", "mountPath": "%s"}]`, istioCertVolumeName, istioCertPath),
		"traffic.sidecar.istio.io/includeInboundPorts":  "",
		"traffic.sidecar.istio.io/includeOutboundPorts": strconv.Itoa(ports.OTLPGRPC),
	}
}
