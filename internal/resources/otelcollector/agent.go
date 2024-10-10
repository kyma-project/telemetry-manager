package otelcollector

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/configchecksum"
	"github.com/kyma-project/telemetry-manager/internal/k8sutils"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
)

const (
	istioCertVolumeName = "istio-certs"
	IstioCertPath       = "/etc/istio-output-certs"
)

type AgentApplierDeleter struct {
	Config AgentConfig
	RBAC   Rbac
}

type AgentApplyOptions struct {
	AllowedPorts        []int32
	CollectorConfigYAML string
}

func (aad *AgentApplierDeleter) ApplyResources(ctx context.Context, c client.Client, opts AgentApplyOptions) error {
	name := types.NamespacedName{Namespace: aad.Config.Namespace, Name: aad.Config.BaseName}

	if err := applyCommonResources(ctx, c, name, aad.RBAC, opts.AllowedPorts); err != nil {
		return fmt.Errorf("failed to create common resource: %w", err)
	}

	configMap := makeConfigMap(name, opts.CollectorConfigYAML)
	if err := k8sutils.CreateOrUpdateConfigMap(ctx, c, configMap); err != nil {
		return fmt.Errorf("failed to create configmap: %w", err)
	}

	configChecksum := configchecksum.Calculate([]corev1.ConfigMap{*configMap}, []corev1.Secret{})
	if err := k8sutils.CreateOrUpdateDaemonSet(ctx, c, aad.makeAgentDaemonSet(configChecksum)); err != nil {
		return fmt.Errorf("failed to create daemonset: %w", err)
	}

	return nil
}

func (aad *AgentApplierDeleter) DeleteResources(ctx context.Context, c client.Client) error {
	// Attempt to clean up as many resources as possible and avoid early return when one of the deletions fails
	var allErrors error = nil

	name := types.NamespacedName{Name: aad.Config.BaseName, Namespace: aad.Config.Namespace}
	if err := deleteCommonResources(ctx, c, name); err != nil {
		allErrors = errors.Join(allErrors, err)
	}

	objectMeta := metav1.ObjectMeta{
		Name:      aad.Config.BaseName,
		Namespace: aad.Config.Namespace,
	}

	configMap := corev1.ConfigMap{ObjectMeta: objectMeta}
	if err := k8sutils.DeleteObject(ctx, c, &configMap); err != nil {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete configmap: %w", err))
	}

	daemonSet := appsv1.DaemonSet{ObjectMeta: objectMeta}
	if err := k8sutils.DeleteObject(ctx, c, &daemonSet); err != nil {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete daemonset: %w", err))
	}

	return allErrors
}

func (aad *AgentApplierDeleter) makeAgentDaemonSet(configChecksum string) *appsv1.DaemonSet {
	selectorLabels := defaultLabels(aad.Config.BaseName)
	podLabels := maps.Clone(selectorLabels)
	podLabels["sidecar.istio.io/inject"] = "true"

	annotations := map[string]string{"checksum/config": configChecksum}
	maps.Copy(annotations, makeIstioTLSPodAnnotations(IstioCertPath))

	dsConfig := aad.Config.DaemonSet
	resources := aad.makeAgentResourceRequirements()
	podSpec := makePodSpec(
		aad.Config.BaseName,
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
			MountPath: IstioCertPath,
			ReadOnly:  true,
		}),
	)

	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      aad.Config.BaseName,
			Namespace: aad.Config.Namespace,
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

func (aad *AgentApplierDeleter) makeAgentResourceRequirements() corev1.ResourceRequirements {
	dsConfig := aad.Config.DaemonSet

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
		"traffic.sidecar.istio.io/includeOutboundPorts":    strconv.Itoa(int(ports.OTLPGRPC)),
		"traffic.sidecar.istio.io/excludeInboundPorts":     strconv.Itoa(int(ports.Metrics)),
		"traffic.sidecar.istio.io/includeOutboundIPRanges": "",
	}
}
