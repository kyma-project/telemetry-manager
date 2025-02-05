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
	metricAgentMemoryLimit   = resource.MustParse("1200Mi")
	metricAgentCPURequest    = resource.MustParse("15m")
	metricAgentMemoryRequest = resource.MustParse("50Mi")
)

func NewMetricAgentApplierDeleter(image, namespace, priorityClassName string) *AgentApplierDeleter {
	extraLabels := map[string]string{
		commonresources.LabelKeyTelemetryMetricScrape: "true",
		commonresources.LabelKeyIstioInject:           "true", // inject Istio sidecar for SDS certificates and agent-to-gateway communication
	}

	return &AgentApplierDeleter{
		baseName:          MetricAgentName,
		extraPodLabel:     extraLabels,
		image:             image,
		namespace:         namespace,
		priorityClassName: priorityClassName,
		rbac:              makeMetricAgentRBAC(namespace),
		memoryLimit:       metricAgentMemoryLimit,
		cpuRequest:        metricAgentCPURequest,
		memoryRequest:     metricAgentMemoryRequest,
	}
}

type AgentApplierDeleter struct {
	baseName          string
	extraPodLabel     map[string]string
	image             string
	namespace         string
	priorityClassName string
	rbac              rbac

	memoryLimit   resource.Quantity
	cpuRequest    resource.Quantity
	memoryRequest resource.Quantity
}

type AgentApplyOptions struct {
	AllowedPorts        []int32
	CollectorConfigYAML string
}

func (aad *AgentApplierDeleter) ApplyResources(ctx context.Context, c client.Client, opts AgentApplyOptions) error {
	name := types.NamespacedName{Namespace: aad.namespace, Name: aad.baseName}

	if err := applyCommonResources(ctx, c, name, commonresources.LabelValueK8sComponentAgent, aad.rbac, opts.AllowedPorts); err != nil {
		return fmt.Errorf("failed to create common resource: %w", err)
	}

	configMap := makeConfigMap(name, commonresources.LabelValueK8sComponentAgent, opts.CollectorConfigYAML)
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

func (aad *AgentApplierDeleter) makeAgentDaemonSet(configChecksum string) *appsv1.DaemonSet {
	annotations := map[string]string{commonresources.AnnotationKeyChecksumConfig: configChecksum}
	maps.Copy(annotations, makeIstioAnnotations(IstioCertPath))

	resources := aad.makeAgentResourceRequirements()

	opts := []podSpecOption{
		commonresources.WithPriorityClass(aad.priorityClassName),
		commonresources.WithResources(resources),
		withEnvVarFromSource(config.EnvVarCurrentPodIP, fieldPathPodIP),
		withEnvVarFromSource(config.EnvVarCurrentNodeName, fieldPathNodeName),
		commonresources.WithGoMemLimitEnvVar(aad.memoryLimit),

		// emptyDir volume for Istio certificates
		withVolume(corev1.Volume{
			Name: istioCertVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		}),
		withVolumeMount(corev1.VolumeMount{
			Name:      istioCertVolumeName,
			MountPath: IstioCertPath,
			ReadOnly:  true,
		}),
	}

	podSpec := makePodSpec(aad.baseName, aad.image, opts...)

	selectorLabels := commonresources.MakeDefaultSelectorLabels(aad.baseName)
	labels := commonresources.MakeDefaultLabels(aad.baseName, commonresources.LabelValueK8sComponentAgent)
	podLabels := make(map[string]string)
	maps.Copy(podLabels, labels)
	maps.Copy(podLabels, aad.extraPodLabel)

	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      aad.baseName,
			Namespace: aad.namespace,
			Labels:    labels,
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
	return corev1.ResourceRequirements{
		Limits: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceMemory: aad.memoryLimit,
		},
		Requests: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    aad.cpuRequest,
			corev1.ResourceMemory: aad.memoryRequest,
		},
	}
}

func makeIstioAnnotations(istioCertPath string) map[string]string {
	// Provision Istio certificates for Prometheus Receiver running as a part of MetricAgent by injecting a sidecar which will rotate SDS certificates and output them to a volume. However, the sidecar should not intercept scraping requests  because Prometheus’s model of direct endpoint access is incompatible with Istio’s sidecar proxy model.
	return map[string]string{
		commonresources.AnnotationKeyIstioProxyConfig: fmt.Sprintf(`# configure an env variable OUTPUT_CERTS to write certificates to the given folder
proxyMetadata:
  OUTPUT_CERTS: %s
`, istioCertPath),
		commonresources.AnnotationKeyIstioUserVolumeMount:         fmt.Sprintf(`[{"name": "%s", "mountPath": "%s"}]`, istioCertVolumeName, istioCertPath),
		commonresources.AnnotationKeyIstioIncludeOutboundPorts:    strconv.Itoa(int(ports.OTLPGRPC)),
		commonresources.AnnotationKeyIstioExcludeInboundPorts:     strconv.Itoa(int(ports.Metrics)),
		commonresources.AnnotationKeyIstioIncludeOutboundIPRanges: "",
	}
}
