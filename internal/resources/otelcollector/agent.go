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

type AgentApplier struct {
	Config AgentConfig
}

type AgentApplyOptions struct {
	AllowedPorts        []int32
	CollectorConfigYAML string
}

func (aa *AgentApplier) ApplyResources(ctx context.Context, c client.Client, opts AgentApplyOptions) error {
	name := types.NamespacedName{Namespace: aa.Config.Namespace, Name: aa.Config.BaseName}

	if err := applyCommonResources(
		ctx,
		c,
		name,
		aa.makeAgentClusterRole(),
		opts.AllowedPorts,
		aa.Config.ObserveBySelfMonitoring,
	); err != nil {
		return fmt.Errorf("failed to create common resource: %w", err)
	}

	configMap := makeConfigMap(name, opts.CollectorConfigYAML)
	if err := k8sutils.CreateOrUpdateConfigMap(ctx, c, configMap); err != nil {
		return fmt.Errorf("failed to create configmap: %w", err)
	}

	configChecksum := configchecksum.Calculate([]corev1.ConfigMap{*configMap}, []corev1.Secret{})
	if err := k8sutils.CreateOrUpdateDaemonSet(ctx, c, aa.makeAgentDaemonSet(configChecksum)); err != nil {
		return fmt.Errorf("failed to create daemonset: %w", err)
	}

	return nil
}

func (aa *AgentApplier) makeAgentClusterRole() *rbacv1.ClusterRole {
	clusterRole := rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      aa.Config.BaseName,
			Namespace: aa.Config.Namespace,
			Labels:    defaultLabels(aa.Config.BaseName),
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

func (aa *AgentApplier) makeAgentDaemonSet(configChecksum string) *appsv1.DaemonSet {
	selectorLabels := defaultLabels(aa.Config.BaseName)
	podLabels := maps.Clone(selectorLabels)
	podLabels["sidecar.istio.io/inject"] = "true"

	annotations := map[string]string{"checksum/config": configChecksum}
	maps.Copy(annotations, makeIstioTLSPodAnnotations(configmetricagent.IstioCertPath))

	dsConfig := aa.Config.DaemonSet
	resources := aa.makeAgentResourceRequirements()
	podSpec := makePodSpec(
		aa.Config.BaseName,
		dsConfig.Image,
		commonresources.WithPriorityClass(dsConfig.PriorityClassName),
		commonresources.WithResources(resources),
		withEnvVarFromSource(config.EnvVarCurrentPodIP, fieldPathPodIP),
		withEnvVarFromSource(config.EnvVarCurrentNodeName, fieldPathNodeName),
		commonresources.WithGoMemLimitEnvVar(dsConfig.MemoryLimit),
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
			Name:      aa.Config.BaseName,
			Namespace: aa.Config.Namespace,
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

func (aa *AgentApplier) makeAgentResourceRequirements() corev1.ResourceRequirements {
	dsConfig := aa.Config.DaemonSet
	return corev1.ResourceRequirements{
		Limits: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    dsConfig.CPULimit,
			corev1.ResourceMemory: dsConfig.MemoryLimit,
		},
		Requests: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    dsConfig.CPURequest,
			corev1.ResourceMemory: dsConfig.MemoryRequest,
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
