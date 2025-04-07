package fluentbit

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/configchecksum"
	"github.com/kyma-project/telemetry-manager/internal/fluentbit/config/builder"
	"github.com/kyma-project/telemetry-manager/internal/fluentbit/ports"
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	k8sutils "github.com/kyma-project/telemetry-manager/internal/utils/k8s"
)

const (
	LogAgentName              = "telemetry-fluent-bit"
	fbSectionsConfigMapName   = LogAgentName + "-sections"
	fbFilesConfigMapName      = LogAgentName + "-files"
	fbLuaConfigMapName        = LogAgentName + "-luascripts"
	fbParsersConfigMapName    = LogAgentName + "-parsers"
	fbEnvConfigSecretName     = LogAgentName + "-env"
	fbTLSFileConfigSecretName = LogAgentName + "-output-tls-config"
	fbDaemonSetName           = LogAgentName
)

var (
	// FluentBit
	fbMemoryLimit   = resource.MustParse("1Gi")
	fbCPURequest    = resource.MustParse("100m")
	fbMemoryRequest = resource.MustParse("50Mi")
)

// AgentApplyOptions expects a syncerClient which is a client with no ownerReference setter since it handles its
// own resource deletion with finalizers and will be removed once the ConfigBuilder implementation is done.
type AgentApplyOptions struct {
	AllowedPorts    []int32
	FluentBitConfig *builder.FluentBitConfig
}

type AgentApplierDeleter struct {
	extraPodLabels    map[string]string
	fluentBitImage    string
	exporterImage     string
	priorityClassName string
	namespace         string

	memoryLimit   resource.Quantity
	cpuRequest    resource.Quantity
	memoryRequest resource.Quantity

	daemonSetName           types.NamespacedName
	luaConfigMapName        types.NamespacedName
	parsersConfigMapName    types.NamespacedName
	filesConfigMapName      types.NamespacedName
	sectionsConfigMapName   types.NamespacedName
	envConfigSecretName     types.NamespacedName
	tlsFileConfigSecretName types.NamespacedName
}

func NewFluentBitApplierDeleter(namespace, fbImage, exporterImage, priorityClassName string) *AgentApplierDeleter {
	return &AgentApplierDeleter{
		namespace: namespace,
		extraPodLabels: map[string]string{
			commonresources.LabelKeyIstioInject:        "true",
			commonresources.LabelKeyTelemetryLogExport: "true",
		},
		fluentBitImage:    fbImage,
		exporterImage:     exporterImage,
		priorityClassName: priorityClassName,

		memoryLimit:   fbMemoryLimit,
		cpuRequest:    fbCPURequest,
		memoryRequest: fbMemoryRequest,

		daemonSetName:           types.NamespacedName{Name: fbDaemonSetName, Namespace: namespace},
		luaConfigMapName:        types.NamespacedName{Name: fbLuaConfigMapName, Namespace: namespace},
		parsersConfigMapName:    types.NamespacedName{Name: fbParsersConfigMapName, Namespace: namespace},
		filesConfigMapName:      types.NamespacedName{Name: fbFilesConfigMapName, Namespace: namespace},
		sectionsConfigMapName:   types.NamespacedName{Name: fbSectionsConfigMapName, Namespace: namespace},
		envConfigSecretName:     types.NamespacedName{Name: fbEnvConfigSecretName, Namespace: namespace},
		tlsFileConfigSecretName: types.NamespacedName{Name: fbTLSFileConfigSecretName, Namespace: namespace},
	}
}

func (aad *AgentApplierDeleter) ApplyResources(ctx context.Context, c client.Client, opts AgentApplyOptions) error {
	serviceAccount := commonresources.MakeServiceAccount(aad.daemonSetName)
	if err := k8sutils.CreateOrUpdateServiceAccount(ctx, c, serviceAccount); err != nil {
		return fmt.Errorf("failed to create fluent bit service account: %w", err)
	}

	clusterRole := makeClusterRole(aad.daemonSetName)
	if err := k8sutils.CreateOrUpdateClusterRole(ctx, c, clusterRole); err != nil {
		return fmt.Errorf("failed to create fluent bit cluster role: %w", err)
	}

	clusterRoleBinding := commonresources.MakeClusterRoleBinding(aad.daemonSetName)
	if err := k8sutils.CreateOrUpdateClusterRoleBinding(ctx, c, clusterRoleBinding); err != nil {
		return fmt.Errorf("failed to create fluent bit cluster role Binding: %w", err)
	}

	exporterMetricsService := makeExporterMetricsService(aad.daemonSetName)
	if err := k8sutils.CreateOrUpdateService(ctx, c, exporterMetricsService); err != nil {
		return fmt.Errorf("failed to reconcile exporter metrics service: %w", err)
	}

	metricsService := makeMetricsService(aad.daemonSetName)
	if err := k8sutils.CreateOrUpdateService(ctx, c, metricsService); err != nil {
		return fmt.Errorf("failed to reconcile fluent bit metrics service: %w", err)
	}

	cm := makeConfigMap(aad.daemonSetName)
	if err := k8sutils.CreateOrUpdateConfigMap(ctx, c, cm); err != nil {
		return fmt.Errorf("failed to reconcile fluent bit configmap: %w", err)
	}

	luaCm := makeLuaConfigMap(aad.luaConfigMapName)
	if err := k8sutils.CreateOrUpdateConfigMap(ctx, c, luaCm); err != nil {
		return fmt.Errorf("failed to reconcile fluent bit lua configmap: %w", err)
	}

	parsersCm := makeParserConfigmap(aad.parsersConfigMapName)
	if err := k8sutils.CreateIfNotExistsConfigMap(ctx, c, parsersCm); err != nil {
		return fmt.Errorf("failed to reconcile fluent bit parsers configmap: %w", err)
	}

	sectionsCm := makeSectionsConfigMap(aad.sectionsConfigMapName, opts.FluentBitConfig.SectionsConfig)
	if err := k8sutils.CreateOrUpdateConfigMap(ctx, c, sectionsCm); err != nil {
		return fmt.Errorf("failed to reconcile fluent bit sections configmap: %w", err)
	}

	filesCm := makeFilesConfigMap(aad.filesConfigMapName, opts.FluentBitConfig.FilesConfig)
	if err := k8sutils.CreateOrUpdateConfigMap(ctx, c, filesCm); err != nil {
		return fmt.Errorf("failed to reconcile fluent bit files configmap: %w", err)
	}

	envConfigSecret := makeEnvConfigSecret(aad.envConfigSecretName, opts.FluentBitConfig.EnvConfigSecret)
	if err := k8sutils.CreateOrUpdateSecret(ctx, c, envConfigSecret); err != nil {
		return fmt.Errorf("failed to reconcile fluent bit env config secret: %w", err)
	}

	tlsFileConfigSecret := makeTLSFileConfigSecret(aad.tlsFileConfigSecretName, opts.FluentBitConfig.TLSConfigSecret)
	if err := k8sutils.CreateOrUpdateSecret(ctx, c, tlsFileConfigSecret); err != nil {
		return fmt.Errorf("failed to reconcile fluent bit tls config secret: %w", err)
	}

	checksum, err := aad.calculateChecksum(ctx, c)
	if err != nil {
		return fmt.Errorf("failed to calculate config checksum: %w", err)
	}

	daemonSet := aad.makeDaemonSet(aad.daemonSetName.Namespace, checksum)
	if err := k8sutils.CreateOrUpdateDaemonSet(ctx, c, daemonSet); err != nil {
		return err
	}

	networkPolicy := commonresources.MakeNetworkPolicy(aad.daemonSetName, opts.AllowedPorts, Labels(), selectorLabels())
	if err := k8sutils.CreateOrUpdateNetworkPolicy(ctx, c, networkPolicy); err != nil {
		return fmt.Errorf("failed to create fluent bit network policy: %w", err)
	}

	return nil
}

func (aad *AgentApplierDeleter) DeleteResources(ctx context.Context, c client.Client) error {
	// Attempt to clean up as many resources as possible and avoid early return when one of the deletions fails
	var allErrors error = nil

	objectMeta := metav1.ObjectMeta{
		Name:      fbDaemonSetName,
		Namespace: aad.namespace,
	}

	serviceAccount := corev1.ServiceAccount{ObjectMeta: objectMeta}
	if err := k8sutils.DeleteObject(ctx, c, &serviceAccount); err != nil {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete serviceaccount: %w", err))
	}

	clusterRole := rbacv1.ClusterRole{ObjectMeta: objectMeta}
	if err := k8sutils.DeleteObject(ctx, c, &clusterRole); err != nil {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete clusterole: %w", err))
	}

	clusterRoleBinding := rbacv1.ClusterRoleBinding{ObjectMeta: objectMeta}
	if err := k8sutils.DeleteObject(ctx, c, &clusterRoleBinding); err != nil {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete clusterolebinding: %w", err))
	}

	exporterMetricsService := corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s-exporter-metrics", fbDaemonSetName), Namespace: aad.namespace}}
	if err := k8sutils.DeleteObject(ctx, c, &exporterMetricsService); err != nil {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete exporter metric service: %w", err))
	}

	metricsService := corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s-metrics", fbDaemonSetName), Namespace: aad.namespace}}
	if err := k8sutils.DeleteObject(ctx, c, &metricsService); err != nil {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete metric service: %w", err))
	}

	cm := corev1.ConfigMap{ObjectMeta: objectMeta}
	if err := k8sutils.DeleteObject(ctx, c, &cm); err != nil {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete configmap: %w", err))
	}

	luaCm := corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name:      fbLuaConfigMapName,
		Namespace: aad.namespace,
	}}
	if err := k8sutils.DeleteObject(ctx, c, &luaCm); err != nil {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete lua configmap: %w", err))
	}

	parserCm := corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name:      fbParsersConfigMapName,
		Namespace: aad.namespace,
	}}
	if err := k8sutils.DeleteObject(ctx, c, &parserCm); err != nil {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete parseopts.Config.ap: %w", err))
	}

	sectionCm := corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name:      fbSectionsConfigMapName,
		Namespace: aad.namespace,
	}}
	if err := k8sutils.DeleteObject(ctx, c, &sectionCm); err != nil {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete section configmap: %w", err))
	}

	filesCm := corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name:      fbFilesConfigMapName,
		Namespace: aad.namespace,
	}}
	if err := k8sutils.DeleteObject(ctx, c, &filesCm); err != nil {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete files configmap: %w", err))
	}

	daemonSet := appsv1.DaemonSet{ObjectMeta: objectMeta}
	if err := k8sutils.DeleteObject(ctx, c, &daemonSet); err != nil {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete daemonset: %w", err))
	}

	networkPolicy := networkingv1.NetworkPolicy{ObjectMeta: objectMeta}
	if err := k8sutils.DeleteObject(ctx, c, &networkPolicy); err != nil {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete networkpolicy: %w", err))
	}

	envSecret := corev1.Secret{ObjectMeta: metav1.ObjectMeta{
		Name:      fbEnvConfigSecretName,
		Namespace: aad.namespace,
	}}
	if err := k8sutils.DeleteObject(ctx, c, &envSecret); err != nil {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete env config secret: %w", err))
	}

	tlsSecret := corev1.Secret{ObjectMeta: metav1.ObjectMeta{
		Name:      fbTLSFileConfigSecretName,
		Namespace: aad.namespace,
	}}
	if err := k8sutils.DeleteObject(ctx, c, &tlsSecret); err != nil {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete tls file config secret: %w", err))
	}

	return allErrors
}

func (aad *AgentApplierDeleter) makeDaemonSet(namespace string, checksum string) *appsv1.DaemonSet {
	ds := &appsv1.DaemonSet{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      LogAgentName,
			Namespace: namespace,
			Labels:    Labels(),
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels(),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						commonresources.AnnotationKeyChecksumConfig:           checksum,
						commonresources.AnnotationKeyIstioExcludeInboundPorts: fmt.Sprintf("%v,%v", ports.HTTP, ports.ExporterMetrics),
					},
					Labels: Labels(),
				},
				Spec: commonresources.MakePodSpec(LogAgentName,
					commonresources.WithPriorityClass(aad.priorityClassName),
					commonresources.WithVolumes(aad.fluentBitVolumes()),
					commonresources.WithContainer("fluent-bit", aad.fluentBitImage,
						commonresources.WithEnvVarsFromSecret(fmt.Sprintf("%s-env", LogAgentName)),
						commonresources.WithPort("http", ports.HTTP),
						commonresources.WithProbes(aad.fluentBitLivenessProbe(), aad.fluentBitReadinessProbe()),
						commonresources.WithResources(aad.fluentBitResources()),
						commonresources.WithVolumeMounts(aad.fluentBitVolumeMounts()),
					),
					commonresources.WithContainer("exporter", aad.exporterImage,
						commonresources.WithArgs([]string{
							"--storage-path=/data/flb-storage/",
							"--metric-name=telemetry_fsbuffer_usage_bytes",
						}),
						commonresources.WithPort("http-metrics", ports.ExporterMetrics),
						commonresources.WithResources(aad.exporterResources()),
						commonresources.WithVolumeMounts(aad.exporterVolumeMounts()),
					),
				),
			},
		},
	}

	return ds
}

func (aad *AgentApplierDeleter) fluentBitResources() corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: aad.memoryLimit,
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    aad.cpuRequest,
			corev1.ResourceMemory: aad.memoryRequest,
		},
	}
}

func (aad *AgentApplierDeleter) exporterResources() corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: aad.memoryLimit,
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    aad.cpuRequest,
			corev1.ResourceMemory: aad.memoryRequest,
		},
	}
}

func (aad *AgentApplierDeleter) fluentBitLivenessProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/",
				Port: intstr.FromString("http"),
			},
		},
	}
}

func (aad *AgentApplierDeleter) fluentBitReadinessProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/api/v1/health",
				Port: intstr.FromString("http"),
			},
		},
	}
}

func (aad *AgentApplierDeleter) fluentBitVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{MountPath: "/fluent-bit/etc", Name: "shared-fluent-bit-config"},
		{MountPath: "/fluent-bit/etc/fluent-bit.conf", Name: "config", SubPath: "fluent-bit.conf"},
		{MountPath: "/fluent-bit/etc/dynamic/", Name: "dynamic-config"},
		{MountPath: "/fluent-bit/etc/dynamic-parsers/", Name: "dynamic-parsers-config"},
		{MountPath: "/fluent-bit/etc/custom_parsers.conf", Name: "config", SubPath: "custom_parsers.conf"},
		{MountPath: "/fluent-bit/scripts/filter-script.lua", Name: "luascripts", SubPath: "filter-script.lua"},
		{MountPath: "/var/log", Name: "varlog", ReadOnly: true},
		{MountPath: "/data", Name: "varfluentbit"},
		{MountPath: "/files", Name: "dynamic-files"},
		{MountPath: "/fluent-bit/etc/output-tls-config/", Name: "output-tls-config", ReadOnly: true},
	}
}

func (aad *AgentApplierDeleter) exporterVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{Name: "varfluentbit", MountPath: "/data"},
	}
}

func (aad *AgentApplierDeleter) fluentBitVolumes() []corev1.Volume {
	return []corev1.Volume{
		{
			Name: "config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: LogAgentName},
				},
			},
		},
		{
			Name: "luascripts",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: fmt.Sprintf("%s-luascripts", LogAgentName)},
				},
			},
		},
		{
			Name: "varlog",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{Path: "/var/log"},
			},
		},
		{
			Name: "shared-fluent-bit-config",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: "dynamic-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: fmt.Sprintf("%s-sections", LogAgentName)},
					Optional:             ptr.To(true),
				},
			},
		},
		{
			Name: "dynamic-parsers-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: fmt.Sprintf("%s-parsers", LogAgentName)},
					Optional:             ptr.To(true),
				},
			},
		},
		{
			Name: "dynamic-files",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: fmt.Sprintf("%s-files", LogAgentName)},
					Optional:             ptr.To(true),
				},
			},
		},
		{
			Name: "varfluentbit",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{Path: fmt.Sprintf("/var/%s", LogAgentName)},
			},
		},
		{
			Name: "output-tls-config",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: fmt.Sprintf("%s-output-tls-config", LogAgentName),
				},
			},
		},
	}
}

func (aad *AgentApplierDeleter) calculateChecksum(ctx context.Context, c client.Client) (string, error) {
	var baseCm corev1.ConfigMap
	if err := c.Get(ctx, aad.daemonSetName, &baseCm); err != nil {
		return "", fmt.Errorf("failed to get %s/%s ConfigMap: %w", aad.daemonSetName.Namespace, aad.daemonSetName.Name, err)
	}

	var parsersCm corev1.ConfigMap
	if err := c.Get(ctx, aad.parsersConfigMapName, &parsersCm); err != nil {
		return "", fmt.Errorf("failed to get %s/%s ConfigMap: %w", aad.parsersConfigMapName.Namespace, aad.parsersConfigMapName.Name, err)
	}

	var luaCm corev1.ConfigMap
	if err := c.Get(ctx, aad.luaConfigMapName, &luaCm); err != nil {
		return "", fmt.Errorf("failed to get %s/%s ConfigMap: %w", aad.luaConfigMapName.Namespace, aad.luaConfigMapName.Name, err)
	}

	var sectionsCm corev1.ConfigMap
	if err := c.Get(ctx, aad.sectionsConfigMapName, &sectionsCm); err != nil {
		return "", fmt.Errorf("failed to get %s/%s ConfigMap: %w", aad.sectionsConfigMapName.Namespace, aad.sectionsConfigMapName.Name, err)
	}

	var filesCm corev1.ConfigMap
	if err := c.Get(ctx, aad.filesConfigMapName, &filesCm); err != nil {
		return "", fmt.Errorf("failed to get %s/%s ConfigMap: %w", aad.filesConfigMapName.Namespace, aad.filesConfigMapName.Name, err)
	}

	var envSecret corev1.Secret
	if err := c.Get(ctx, aad.envConfigSecretName, &envSecret); err != nil {
		return "", fmt.Errorf("failed to get %s/%s Secret: %w", aad.envConfigSecretName.Namespace, aad.envConfigSecretName.Name, err)
	}

	var tlsSecret corev1.Secret
	if err := c.Get(ctx, aad.tlsFileConfigSecretName, &tlsSecret); err != nil {
		return "", fmt.Errorf("failed to get %s/%s Secret: %w", aad.tlsFileConfigSecretName.Namespace, aad.tlsFileConfigSecretName.Name, err)
	}

	return configchecksum.Calculate([]corev1.ConfigMap{baseCm, parsersCm, luaCm, sectionsCm, filesCm}, []corev1.Secret{envSecret, tlsSecret}), nil
}

func makeClusterRole(name types.NamespacedName) *rbacv1.ClusterRole {
	clusterRole := rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
			Labels:    Labels(),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"namespaces", "pods"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	}

	return &clusterRole
}

func makeMetricsService(name types.NamespacedName) *corev1.Service {
	serviceLabels := Labels()
	serviceLabels[commonresources.LabelKeyTelemetrySelfMonitor] = commonresources.LabelValueTelemetrySelfMonitor

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-metrics", name.Name),
			Namespace: name.Namespace,
			Labels:    serviceLabels,
			Annotations: map[string]string{
				commonresources.AnnotationKeyPrometheusScrape: "true",
				commonresources.AnnotationKeyPrometheusPort:   strconv.Itoa(ports.HTTP),
				commonresources.AnnotationKeyPrometheusScheme: "http",
				commonresources.AnnotationKeyPrometheusPath:   "/api/v2/metrics/prometheus",
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Protocol:   "TCP",
					Port:       int32(ports.HTTP),
					TargetPort: intstr.FromString("http"),
				},
			},
			Selector: selectorLabels(),
			Type:     corev1.ServiceTypeClusterIP,
		},
	}
}

func makeExporterMetricsService(name types.NamespacedName) *corev1.Service {
	serviceLabels := Labels()
	serviceLabels[commonresources.LabelKeyTelemetrySelfMonitor] = commonresources.LabelValueTelemetrySelfMonitor

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-exporter-metrics", name.Name),
			Namespace: name.Namespace,
			Labels:    serviceLabels,
			Annotations: map[string]string{
				commonresources.AnnotationKeyPrometheusScrape: "true",
				commonresources.AnnotationKeyPrometheusPort:   strconv.Itoa(ports.ExporterMetrics),
				commonresources.AnnotationKeyPrometheusScheme: "http",
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "http-metrics",
					Protocol:   "TCP",
					Port:       int32(ports.ExporterMetrics),
					TargetPort: intstr.FromString("http-metrics"),
				},
			},
			Selector: selectorLabels(),
			Type:     corev1.ServiceTypeClusterIP,
		},
	}
}

func makeConfigMap(name types.NamespacedName) *corev1.ConfigMap {
	parserConfig := `
[PARSER]
    Name docker_no_time
    Format json
    Time_Keep Off
    Time_Key time
    Time_Format %Y-%m-%dT%H:%M:%S.%L
`

	fluentBitConfig := `
[SERVICE]
    Daemon Off
    Flush 1
    Log_Level warn
    Parsers_File custom_parsers.conf
    Parsers_File dynamic-parsers/parsers.conf
    HTTP_Server On
    HTTP_Listen 0.0.0.0
    HTTP_Port {{ HTTP_PORT }}
    storage.path /data/flb-storage/
    storage.metrics on

@INCLUDE dynamic/*.conf
`
	fluentBitConfig = strings.Replace(fluentBitConfig, "{{ HTTP_PORT }}", strconv.Itoa(ports.HTTP), 1)

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
			Labels:    Labels(),
		},
		Data: map[string]string{
			"custom_parsers.conf": parserConfig,
			"fluent-bit.conf":     fluentBitConfig,
		},
	}
}

func makeSectionsConfigMap(name types.NamespacedName, sectionsConfig map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
			Labels:    Labels(),
		},
		Data: sectionsConfig,
	}
}

func makeFilesConfigMap(name types.NamespacedName, filesConfig map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
			Labels:    Labels(),
		},
		Data: filesConfig,
	}
}

func makeParserConfigmap(name types.NamespacedName) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
			Labels:    Labels(),
		},
		Data: map[string]string{"parsers.conf": ""},
	}
}

func makeLuaConfigMap(name types.NamespacedName) *corev1.ConfigMap {
	//nolint:dupword // Ignore lua syntax code duplications.
	luaFilter := `
function kubernetes_map_keys(tag, timestamp, record)
  if record.kubernetes == nil then
    return 0
  end
  map_keys(record.kubernetes.annotations)
  map_keys(record.kubernetes.labels)
  return 1, timestamp, record
end
function map_keys(table)
  if table == nil then
    return
  end
  local new_table = {}
  local changed_keys = {}
  for key, val in pairs(table) do
    local mapped_key = string.gsub(key, "[%/%.]", "_")
    if mapped_key ~= key then
      new_table[mapped_key] = val
      changed_keys[key] = true
    end
  end
  for key in pairs(changed_keys) do
    table[key] = nil
  end
  for key, val in pairs(new_table) do
    table[key] = val
  end
end
`

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
			Labels:    Labels(),
		},
		Data: map[string]string{"filter-script.lua": luaFilter},
	}
}

func makeEnvConfigSecret(name types.NamespacedName, envConfigSecret map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
			Labels:    Labels(),
		},
		Data: envConfigSecret,
	}
}

func makeTLSFileConfigSecret(name types.NamespacedName, tlsFileConfigSecret map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
			Labels:    Labels(),
		},
		Data: tlsFileConfigSecret,
	}
}

func Labels() map[string]string {
	result := commonresources.MakeDefaultLabels("fluent-bit", commonresources.LabelValueK8sComponentAgent)
	result[commonresources.LabelKeyK8sInstance] = commonresources.LabelValueK8sInstance

	return result
}

func selectorLabels() map[string]string {
	result := commonresources.MakeDefaultSelectorLabels("fluent-bit")
	result[commonresources.LabelKeyK8sInstance] = commonresources.LabelValueK8sInstance

	return result
}
