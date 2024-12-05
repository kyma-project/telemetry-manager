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
	"github.com/kyma-project/telemetry-manager/internal/labels"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	k8sutils "github.com/kyma-project/telemetry-manager/internal/utils/k8s"
)

const (
	IstioCertPath   = "/etc/istio-output-certs"
	MetricAgentName = "telemetry-metric-agent"

	istioCertVolumeName = "istio-certs"
)

var (
	metricAgentCPULimit      = resource.MustParse("1")
	metricAgentMemoryLimit   = resource.MustParse("1200Mi")
	metricAgentCPURequest    = resource.MustParse("15m")
	metricAgentMemoryRequest = resource.MustParse("50Mi")
)

func NewMetricAgentApplierDeleter(image, namespace, priorityClassName string) *AgentApplierDeleter {
	return &AgentApplierDeleter{
		baseName:          MetricAgentName,
		image:             image,
		namespace:         namespace,
		priorityClassName: priorityClassName,
		rbac:              makeMetricAgentRBAC(namespace),
		cpuLimit:          metricAgentCPULimit,
		memoryLimit:       metricAgentMemoryLimit,
		cpuRequest:        metricAgentCPURequest,
		memoryRequest:     metricAgentMemoryRequest,
	}
}

type AgentApplierDeleter struct {
	baseName          string
	image             string
	namespace         string
	priorityClassName string
	rbac              rbac

	cpuLimit      resource.Quantity
	memoryLimit   resource.Quantity
	cpuRequest    resource.Quantity
	memoryRequest resource.Quantity
}

type AgentApplyOptions struct {
	AllowedPorts            []int32
	CollectorConfigYAML     string
	ComponentSelectorLabels map[string]string
}

func (aad *AgentApplierDeleter) ApplyResources(ctx context.Context, c client.Client, opts AgentApplyOptions) error {
	name := types.NamespacedName{Namespace: aad.namespace, Name: aad.baseName}

	if err := applyCommonResources(ctx, c, name, aad.rbac, opts.AllowedPorts); err != nil {
		return fmt.Errorf("failed to create common resource: %w", err)
	}

	configMap := makeConfigMap(name, opts.CollectorConfigYAML)
	if err := k8sutils.CreateOrUpdateConfigMap(ctx, c, configMap); err != nil {
		return fmt.Errorf("failed to create configmap: %w", err)
	}

	configChecksum := configchecksum.Calculate([]corev1.ConfigMap{*configMap}, []corev1.Secret{})
	if err := k8sutils.CreateOrUpdateDaemonSet(ctx, c, aad.makeAgentDaemonSet(configChecksum, opts)); err != nil {
		return fmt.Errorf("failed to create daemonset: %w", err)
	}

	return nil
}

func (aad *AgentApplierDeleter) DeleteResources(ctx context.Context, c client.Client) error {
	// Attempt to clean up as many resources as possible and avoid early return when one of the deletions fails
	var allErrors error = nil

	name := types.NamespacedName{Name: aad.baseName, Namespace: aad.namespace}
	if err := deleteCommonResources(ctx, c, name); err != nil {
		allErrors = errors.Join(allErrors, err)
	}

	objectMeta := metav1.ObjectMeta{
		Name:      aad.baseName,
		Namespace: aad.namespace,
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

func (aad *AgentApplierDeleter) makeAgentDaemonSet(configChecksum string, opts AgentApplyOptions) *appsv1.DaemonSet {
	selectorLabels := labels.MakeDefaultLabel(aad.baseName)

	annotations := map[string]string{"checksum/config": configChecksum}
	maps.Copy(annotations, makeIstioTLSPodAnnotations(IstioCertPath))

	resources := aad.makeAgentResourceRequirements()
	podSpec := makePodSpec(
		aad.baseName,
		aad.image,
		commonresources.WithPriorityClass(aad.priorityClassName),
		commonresources.WithResources(resources),
		withEnvVarFromSource(config.EnvVarCurrentPodIP, fieldPathPodIP),
		withEnvVarFromSource(config.EnvVarCurrentNodeName, fieldPathNodeName),
		commonresources.WithGoMemLimitEnvVar(aad.memoryLimit),
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
			Name:      aad.baseName,
			Namespace: aad.namespace,
			Labels:    selectorLabels,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      opts.ComponentSelectorLabels,
					Annotations: annotations,
				},
				Spec: podSpec,
			},
		},
	}
}

func (aad *AgentApplierDeleter) makeAgentResourceRequirements() corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Limits: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    aad.cpuLimit,
			corev1.ResourceMemory: aad.memoryLimit,
		},
		Requests: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    aad.cpuRequest,
			corev1.ResourceMemory: aad.memoryRequest,
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
