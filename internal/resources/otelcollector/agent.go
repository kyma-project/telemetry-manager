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
	"github.com/kyma-project/telemetry-manager/internal/k8sutils"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	configmetricagent "github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric/agent"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
)

const istioCertVolumeName = "istio-certs"

func ApplyAgentResources(ctx context.Context, c client.Client, cfg *AgentConfig) error {
	name := types.NamespacedName{Namespace: cfg.Namespace, Name: cfg.BaseName}

	if err := applyCommonResources(ctx, c, name, makeAgentClusterRole(name), cfg.allowedPorts); err != nil {
		return fmt.Errorf("failed to create common resource: %w", err)
	}

	configMap := makeConfigMap(name, cfg.CollectorConfig)
	if err := k8sutils.CreateOrUpdateConfigMap(ctx, c, configMap); err != nil {
		return fmt.Errorf("failed to create configmap: %w", err)
	}

	configChecksum := configchecksum.Calculate([]corev1.ConfigMap{*configMap}, []corev1.Secret{})
	if err := k8sutils.CreateOrUpdateDaemonSet(ctx, c, makeAgentDaemonSet(cfg, configChecksum)); err != nil {
		return fmt.Errorf("failed to create daemonset: %w", err)
	}

	return nil
}

func makeAgentClusterRole(name types.NamespacedName) *rbacv1.ClusterRole {
	clusterRole := rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
			Labels:    defaultLabels(name.Name),
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

	annotations := map[string]string{"checksum/config": configChecksum}
	maps.Copy(annotations, makeIstioTLSPodAnnotations(configmetricagent.IstioCertPath))

	resources := makeAgentResourceRequirements(cfg)
	podSpec := makePodSpec(cfg.BaseName, cfg.DaemonSet.Image,
		commonresources.WithPriorityClass(cfg.DaemonSet.PriorityClassName),
		commonresources.WithResources(resources),
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
			Labels:    selectorLabels,
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
		"sidecar.istio.io/userVolumeMount":                 fmt.Sprintf(`[{"name": "%s", "mountPath": "%s"}]`, istioCertVolumeName, istioCertPath),
		"traffic.sidecar.istio.io/includeOutboundPorts":    strconv.Itoa(ports.OTLPGRPC),
		"traffic.sidecar.istio.io/excludeInboundPorts":     strconv.Itoa(ports.Metrics),
		"traffic.sidecar.istio.io/includeOutboundIPRanges": "",
	}
}
