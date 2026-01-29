package otelcollector

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/config"
	"github.com/kyma-project/telemetry-manager/internal/configchecksum"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	"github.com/kyma-project/telemetry-manager/internal/resources/names"
	k8sutils "github.com/kyma-project/telemetry-manager/internal/utils/k8s"
)

const (
	IstioCertPath       = "/etc/istio-output-certs"
	istioCertVolumeName = "istio-certs"

	checkpointVolumeName = "tmp"
	CheckpointVolumePath = "/tmp"
	logVolumeName        = "varlogpods"
	logVolumePath        = "/var/log/pods"
)

var (
	metricAgentCPURequest    = resource.MustParse("15m")
	metricAgentMemoryRequest = resource.MustParse("50Mi")
	metricAgentMemoryLimit   = resource.MustParse("1200Mi")

	logAgentCPURequest    = resource.MustParse("15m")
	logAgentMemoryRequest = resource.MustParse("50Mi")
	logAgentMemoryLimit   = resource.MustParse("1200Mi")
)

type AgentApplierDeleter struct {
	globals config.Global

	baseName            string
	extraPodLabel       map[string]string
	makeAnnotationsFunc func(configChecksum string, opts AgentApplyOptions) map[string]string
	image               string
	rbac                rbac

	podOpts       []commonresources.PodSpecOption
	containerOpts []commonresources.ContainerOption
}

type AgentApplyOptions struct {
	IstioEnabled        bool
	CollectorConfigYAML string
	CollectorEnvVars    map[string][]byte
	// BackendPorts is needed only for the metric agent to set the value of the annotation "traffic.sidecar.istio.io/includeOutboundPorts"
	BackendPorts []string
}

func NewLogAgentApplierDeleter(globals config.Global, collectorImage, priorityClassName string) *AgentApplierDeleter {
	extraLabels := map[string]string{
		commonresources.LabelKeyIstioInject: commonresources.LabelValueTrue, // inject Istio sidecar
	}

	volumes := []corev1.Volume{
		makePodLogsVolume(),
		// HostPath Should be unique for each application using it
		makeFileLogCheckpointVolume(),
	}

	collectorVolumeMounts := []corev1.VolumeMount{
		makePodLogsVolumeMount(),
		makeFileLogCheckPointVolumeMount(),
	}

	collectorResources := commonresources.MakeResourceRequirements(
		logAgentMemoryLimit,
		logAgentMemoryRequest,
		logAgentCPURequest,
	)

	return &AgentApplierDeleter{
		globals:             globals,
		baseName:            names.LogAgent,
		extraPodLabel:       extraLabels,
		makeAnnotationsFunc: makeLogAgentAnnotations,
		image:               collectorImage,
		rbac:                makeLogAgentRBAC(globals.TargetNamespace()),
		podOpts: []commonresources.PodSpecOption{
			commonresources.WithPriorityClass(priorityClassName),
			commonresources.WithVolumes(volumes),
		},
		containerOpts: []commonresources.ContainerOption{
			commonresources.WithResources(collectorResources),
			commonresources.WithEnvVarFromField(common.EnvVarCurrentPodIP, fieldPathPodIP),
			commonresources.WithGoMemLimitEnvVar(logAgentMemoryLimit),
			commonresources.WithFIPSGoDebugEnvVar(globals.OperateInFIPSMode()),
			commonresources.WithVolumeMounts(collectorVolumeMounts),
			commonresources.WithRunAsGroup(commonresources.GroupRoot),
			commonresources.WithRunAsUser(commonresources.UserDefault),
		},
	}
}

func NewMetricAgentApplierDeleter(globals config.Global, image, priorityClassName string) *AgentApplierDeleter {
	extraLabels := map[string]string{
		commonresources.LabelKeyTelemetryMetricScrape:    commonresources.LabelValueTrue,
		commonresources.LabelKeyTelemetryMetricExport:    commonresources.LabelValueTrue,
		commonresources.LabelKeyIstioInject:              commonresources.LabelValueTrue, // inject Istio sidecar
		commonresources.LabelKeyTelemetryMetricsScraping: commonresources.LabelValueTelemetryMetricsScraping,
	}

	return &AgentApplierDeleter{
		globals:             globals,
		baseName:            names.MetricAgent,
		extraPodLabel:       extraLabels,
		makeAnnotationsFunc: makeMetricAgentAnnotations,
		image:               image,
		rbac:                makeMetricAgentRBAC(globals.TargetNamespace()),
		podOpts: []commonresources.PodSpecOption{
			commonresources.WithPriorityClass(priorityClassName),
			commonresources.WithVolumes([]corev1.Volume{makeIstioCertVolume()}),
		},
		containerOpts: []commonresources.ContainerOption{
			commonresources.WithEnvVarFromField(common.EnvVarCurrentPodIP, fieldPathPodIP),
			commonresources.WithEnvVarFromField(common.EnvVarCurrentNodeName, fieldPathNodeName),
			commonresources.WithGoMemLimitEnvVar(metricAgentMemoryLimit),
			commonresources.WithFIPSGoDebugEnvVar(globals.OperateInFIPSMode()),
			commonresources.WithResources(commonresources.MakeResourceRequirements(
				metricAgentMemoryLimit,
				metricAgentMemoryRequest,
				metricAgentCPURequest,
			)),
			commonresources.WithVolumeMounts([]corev1.VolumeMount{makeIstioCertVolumeMount()}),
		},
	}
}

func (aad *AgentApplierDeleter) ApplyResources(ctx context.Context, c client.Client, opts AgentApplyOptions) error {
	name := types.NamespacedName{Namespace: aad.globals.TargetNamespace(), Name: aad.baseName}

	ingressAllowedPorts := agentIngressAllowedPorts()
	if opts.IstioEnabled {
		ingressAllowedPorts = append(ingressAllowedPorts, ports.IstioEnvoy)
	}

	if err := applyCommonResources(ctx, c, name, commonresources.LabelValueK8sComponentAgent, aad.rbac, ingressAllowedPorts); err != nil {
		return fmt.Errorf("failed to create common resource: %w", err)
	}

	secretsInChecksum := []corev1.Secret{}

	if opts.CollectorEnvVars != nil {
		secret := makeSecret(name, commonresources.LabelValueK8sComponentAgent, opts.CollectorEnvVars)
		if err := k8sutils.CreateOrUpdateSecret(ctx, c, secret); err != nil {
			return fmt.Errorf("failed to create env secret: %w", err)
		}

		secretsInChecksum = append(secretsInChecksum, *secret)
	}

	configMap := makeConfigMap(name, commonresources.LabelValueK8sComponentAgent, opts.CollectorConfigYAML)
	if err := k8sutils.CreateOrUpdateConfigMap(ctx, c, configMap); err != nil {
		return fmt.Errorf("failed to create configmap: %w", err)
	}

	configChecksum := configchecksum.Calculate([]corev1.ConfigMap{*configMap}, secretsInChecksum)
	if err := k8sutils.CreateOrUpdateDaemonSet(ctx, c, aad.makeAgentDaemonSet(configChecksum, opts)); err != nil {
		return fmt.Errorf("failed to create daemonset: %w", err)
	}

	return nil
}

func (aad *AgentApplierDeleter) DeleteResources(ctx context.Context, c client.Client) error {
	// Attempt to clean up as many resources as possible and avoid early return when one of the deletions fails
	var allErrors error = nil

	name := types.NamespacedName{Name: aad.baseName, Namespace: aad.globals.TargetNamespace()}
	if err := deleteCommonResources(ctx, c, name); err != nil {
		allErrors = errors.Join(allErrors, err)
	}

	objectMeta := metav1.ObjectMeta{
		Name:      aad.baseName,
		Namespace: aad.globals.TargetNamespace(),
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
	annotations := aad.makeAnnotationsFunc(configChecksum, opts)

	// Add pod options shared between all agents
	podOpts := slices.Clone(aad.podOpts)
	podOpts = append(podOpts, commonresources.WithTolerations(commonresources.CriticalDaemonSetTolerations))
	podOpts = append(podOpts, commonresources.WithImagePullSecretName(aad.globals.ImagePullSecretName()))
	podOpts = append(podOpts, commonresources.WithClusterTrustBundleVolume(aad.globals.ClusterTrustBundleName()))

	containerOpts := slices.Clone(aad.containerOpts)
	containerOpts = append(containerOpts, commonresources.WithClusterTrustBundleVolumeMount(aad.globals.ClusterTrustBundleName()))

	podSpec := makePodSpec(aad.baseName, aad.image, podOpts, containerOpts)

	metadata := MakeWorkloadMetadata(
		&aad.globals,
		aad.baseName,
		commonresources.LabelValueK8sComponentAgent,
		aad.extraPodLabel,
		annotations,
	)

	return makeDaemonSet(
		aad.baseName,
		aad.globals.TargetNamespace(),
		metadata,
		podSpec,
	)
}

func makeLogAgentAnnotations(configChecksum string, opts AgentApplyOptions) map[string]string {
	annotations := map[string]string{commonresources.AnnotationKeyChecksumConfig: configChecksum}

	if opts.IstioEnabled {
		annotations[commonresources.AnnotationKeyIstioExcludeInboundPorts] = strconv.Itoa(int(ports.Metrics))
	}

	return annotations
}

func makeMetricAgentAnnotations(configChecksum string, opts AgentApplyOptions) map[string]string {
	annotations := map[string]string{commonresources.AnnotationKeyChecksumConfig: configChecksum}

	if opts.IstioEnabled {
		annotations[commonresources.AnnotationKeyIstioExcludeInboundPorts] = strconv.Itoa(int(ports.Metrics))
		// Provision Istio certificates for Prometheus Receiver running as a part of MetricAgent by injecting a sidecar which will rotate SDS certificates and output them to a volume.
		annotations[commonresources.AnnotationKeyIstioProxyConfig] = fmt.Sprintf(`# configure an env variable OUTPUT_CERTS to write certificates to the given folder
proxyMetadata:
  OUTPUT_CERTS: %s
`, IstioCertPath)
		annotations[commonresources.AnnotationKeyIstioUserVolumeMount] = fmt.Sprintf(`[{"name": "%s", "mountPath": "%s"}]`, istioCertVolumeName, IstioCertPath)
		// The Istio sidecar should not intercept scraping requests  because Prometheus’s model of direct endpoint access is incompatible with Istio’s sidecar proxy model.
		// So, all outbound traffic should bypass Istio’s sidecar (traffic.sidecar.istio.io/includeOutboundIPRanges: "") with the exception of the traffic to the backends (traffic.sidecar.istio.io/includeOutboundPorts: {BACKEND_PORT_1},{BACKEND_PORT_2})
		// For more details, check the ADR: https://github.com/kyma-project/telemetry-manager/blob/main/docs/contributor/arch/026-istio-outgoing-communication-for-metric-agent.md
		annotations[commonresources.AnnotationKeyIstioIncludeOutboundIPRanges] = ""
		annotations[commonresources.AnnotationKeyIstioIncludeOutboundPorts] = strings.Join(opts.BackendPorts, ",")
	}

	return annotations
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

func agentIngressAllowedPorts() []int32 {
	return []int32{
		ports.Metrics,
		ports.HealthCheck,
	}
}
