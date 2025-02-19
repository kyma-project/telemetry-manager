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
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/configchecksum"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	k8sutils "github.com/kyma-project/telemetry-manager/internal/utils/k8s"
)

const (
	IstioCertPath       = "/etc/istio-output-certs"
	istioCertVolumeName = "istio-certs"

	MetricAgentName = "telemetry-metric-agent"
	LogAgentName    = "telemetry-log-agent"

	checkpointVolumeName = "varlibtelemetrylogagent"
	CheckpointVolumePath = "/var/lib/telemetry-log-agent"
	logVolumeName        = "varlogpods"
	logVolumePath        = "/var/log/pods"
)

var (
	metricAgentMemoryLimit   = resource.MustParse("1200Mi")
	metricAgentCPURequest    = resource.MustParse("15m")
	metricAgentMemoryRequest = resource.MustParse("50Mi")

	logAgentMemoryLimit   = resource.MustParse("1200Mi")
	logAgentCPURequest    = resource.MustParse("15m")
	logAgentMemoryRequest = resource.MustParse("50Mi")
)

type AgentApplierDeleter struct {
	baseName      string
	extraPodLabel map[string]string
	image         string
	namespace     string
	rbac          rbac

	podSpecOptions []podSpecOption
}

type AgentApplyOptions struct {
	AllowedPorts        []int32
	CollectorConfigYAML string
}

func NewLogAgentApplierDeleter(image, namespace, priorityClassName string) *AgentApplierDeleter {
	extraLabels := map[string]string{
		commonresources.LabelKeyIstioInject: "true", // inject Istio sidecar for SDS certificates and agent-to-gateway communication
	}
	volumes := []corev1.Volume{
		makeIstioCertVolume(),
		makePodLogsVolume(),
		makeFileLogCheckpointVolume(),
	}
	volumeMounts := []corev1.VolumeMount{
		makeIstioCertVolumeMount(),
		makePodLogsVolumeMount(),
		makeFileLogCheckPointVolumeMount(),
	}

	return &AgentApplierDeleter{
		baseName:      LogAgentName,
		extraPodLabel: extraLabels,
		image:         image,
		namespace:     namespace,

		podSpecOptions: []podSpecOption{
			commonresources.WithPriorityClass(priorityClassName),
			commonresources.WithResources(makeAgentResourceRequirements(logAgentMemoryLimit, logAgentMemoryRequest, logAgentCPURequest)),
			withEnvVarFromSource(config.EnvVarCurrentPodIP, fieldPathPodIP),
			commonresources.WithGoMemLimitEnvVar(metricAgentMemoryLimit),
			withVolumes(volumes),
			withVolumeMounts(volumeMounts),
			withSecurityContext(makeLogAgentSecurityContext()),
			withPodSecurityContext(makeLogAgentPodSecurityContext()),
		},
	}
}

func NewMetricAgentApplierDeleter(image, namespace, priorityClassName string) *AgentApplierDeleter {
	extraLabels := map[string]string{
		commonresources.LabelKeyTelemetryMetricScrape: "true",
		commonresources.LabelKeyIstioInject:           "true", // inject Istio sidecar for SDS certificates and agent-to-gateway communication
	}

	return &AgentApplierDeleter{
		baseName:      MetricAgentName,
		extraPodLabel: extraLabels,
		image:         image,
		namespace:     namespace,
		rbac:          makeMetricAgentRBAC(namespace),

		podSpecOptions: []podSpecOption{
			commonresources.WithPriorityClass(priorityClassName),
			commonresources.WithResources(makeAgentResourceRequirements(metricAgentMemoryLimit, metricAgentMemoryRequest, metricAgentCPURequest)),
			withEnvVarFromSource(config.EnvVarCurrentPodIP, fieldPathPodIP),
			withEnvVarFromSource(config.EnvVarCurrentNodeName, fieldPathNodeName),
			commonresources.WithGoMemLimitEnvVar(metricAgentMemoryLimit),
			withVolumes([]corev1.Volume{makeIstioCertVolume()}),
			withVolumeMounts([]corev1.VolumeMount{makeIstioCertVolumeMount()}),
			withSecurityContext(makeMetricAgentSecurityContext()),
			withPodSecurityContext(makeMetricAgentPodSecurityContext()),
		},
	}
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

	podSpec := makePodSpec(aad.baseName, aad.image, aad.podSpecOptions...)

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

func makeAgentResourceRequirements(memoryLimit, memoryRequest, cpuRequest resource.Quantity) corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Limits: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceMemory: memoryLimit,
		},
		Requests: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    cpuRequest,
			corev1.ResourceMemory: memoryRequest,
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

func makeIstioCertVolume() corev1.Volume {
	// emptyDir volume for Istio certificates
	return corev1.Volume{
		Name: istioCertVolumeName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
}

func makeIstioCertVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      istioCertVolumeName,
		MountPath: IstioCertPath,
		ReadOnly:  true,
	}
}

func makePodLogsVolume() corev1.Volume {
	return corev1.Volume{
		Name: logVolumeName,
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: logVolumePath,
				Type: nil,
			},
		},
	}
}

func makePodLogsVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      logVolumeName,
		MountPath: logVolumePath,
		ReadOnly:  true,
	}
}

func makeFileLogCheckpointVolume() corev1.Volume {
	return corev1.Volume{
		Name: checkpointVolumeName,
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: CheckpointVolumePath,
				Type: ptr.To(corev1.HostPathDirectoryOrCreate),
			},
		},
	}
}

func makeFileLogCheckPointVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      checkpointVolumeName,
		MountPath: CheckpointVolumePath,
	}
}

func makeMetricAgentSecurityContext() *corev1.SecurityContext {
	return &corev1.SecurityContext{
		Privileged:               ptr.To(false),
		RunAsUser:                ptr.To(collectorUser),
		RunAsNonRoot:             ptr.To(true),
		ReadOnlyRootFilesystem:   ptr.To(true),
		AllowPrivilegeEscalation: ptr.To(false),
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		},
	}
}

func makeLogAgentSecurityContext() *corev1.SecurityContext {
	return &corev1.SecurityContext{
		Privileged:               ptr.To(false),
		RunAsUser:                ptr.To(int64(0)),
		RunAsNonRoot:             ptr.To(false),
		ReadOnlyRootFilesystem:   ptr.To(true),
		AllowPrivilegeEscalation: ptr.To(false),
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
		Capabilities: &corev1.Capabilities{
			Add:  []corev1.Capability{"FOWNER"},
			Drop: []corev1.Capability{"ALL"},
		},
	}
}

func makeMetricAgentPodSecurityContext() *corev1.PodSecurityContext {
	return &corev1.PodSecurityContext{
		RunAsUser:    ptr.To(collectorUser),
		RunAsNonRoot: ptr.To(true),
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
	}
}

func makeLogAgentPodSecurityContext() *corev1.PodSecurityContext {
	return &corev1.PodSecurityContext{
		RunAsNonRoot: ptr.To(false),
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
	}
}
