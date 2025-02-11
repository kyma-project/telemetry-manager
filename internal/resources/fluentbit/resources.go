package fluentbit

import (
	"context"
	"errors"
	"fmt"
	"maps"
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

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/configchecksum"
	"github.com/kyma-project/telemetry-manager/internal/fluentbit/config/builder"
	"github.com/kyma-project/telemetry-manager/internal/fluentbit/ports"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
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

type Config struct {
	PipelineDefaults builder.PipelineDefaults
	Overrides        overrides.Config
}

type ResourceNames struct {
	DaemonSet           types.NamespacedName
	LuaConfigMap        types.NamespacedName
	ParsersConfigMap    types.NamespacedName
	SectionsConfigMap   types.NamespacedName
	FilesConfigMap      types.NamespacedName
	EnvConfigSecret     types.NamespacedName
	TLSFileConfigSecret types.NamespacedName
}

type AgentApplyOptions struct {
	Config                 Config
	AllowedPorts           []int32
	Pipeline               *telemetryv1alpha1.LogPipeline
	DeployableLogPipelines []telemetryv1alpha1.LogPipeline
}

type AgentApplierDeleter struct {
	extraPodLabel     map[string]string
	fluentBitImage    string
	exporterImage     string
	priorityClassName string
	namespace         string

	memoryLimit   resource.Quantity
	cpuRequest    resource.Quantity
	memoryRequest resource.Quantity
}

func NewFluentBitApplierDeleter(namespace, fbImage, exporterImage, priorityClassName string) *AgentApplierDeleter {
	return &AgentApplierDeleter{
		namespace: namespace,
		extraPodLabel: map[string]string{
			commonresources.LabelKeyIstioInject:        "true",
			commonresources.LabelKeyTelemetryLogExport: "true",
		},
		fluentBitImage:    fbImage,
		exporterImage:     exporterImage,
		priorityClassName: priorityClassName,

		memoryLimit:   fbMemoryLimit,
		cpuRequest:    fbCPURequest,
		memoryRequest: fbMemoryRequest,
	}
}

func (aad *AgentApplierDeleter) ApplyResources(ctx context.Context, c client.Client, opts AgentApplyOptions) error {
	names := ResourceNames{
		DaemonSet:           types.NamespacedName{Name: fbDaemonSetName, Namespace: aad.namespace},
		LuaConfigMap:        types.NamespacedName{Name: fbLuaConfigMapName, Namespace: aad.namespace},
		ParsersConfigMap:    types.NamespacedName{Name: fbParsersConfigMapName, Namespace: aad.namespace},
		SectionsConfigMap:   types.NamespacedName{Name: fbSectionsConfigMapName, Namespace: aad.namespace},
		FilesConfigMap:      types.NamespacedName{Name: fbFilesConfigMapName, Namespace: aad.namespace},
		EnvConfigSecret:     types.NamespacedName{Name: fbEnvConfigSecretName, Namespace: aad.namespace},
		TLSFileConfigSecret: types.NamespacedName{Name: fbTLSFileConfigSecretName, Namespace: aad.namespace},
	}

	syncer := syncer{
		Client: c,
		Config: opts.Config,
		Names:  names,
	}

	if err := syncer.syncFluentBitConfig(ctx, opts.Pipeline, opts.DeployableLogPipelines); err != nil {
		return fmt.Errorf("failed to sync fluent bit config maps: %w", err)
	}

	serviceAccount := commonresources.MakeServiceAccount(names.DaemonSet)
	if err := k8sutils.CreateOrUpdateServiceAccount(ctx, c, serviceAccount); err != nil {
		return fmt.Errorf("failed to create fluent bit service account: %w", err)
	}

	clusterRole := makeClusterRole(names.DaemonSet)
	if err := k8sutils.CreateOrUpdateClusterRole(ctx, c, clusterRole); err != nil {
		return fmt.Errorf("failed to create fluent bit cluster role: %w", err)
	}

	clusterRoleBinding := commonresources.MakeClusterRoleBinding(names.DaemonSet)
	if err := k8sutils.CreateOrUpdateClusterRoleBinding(ctx, c, clusterRoleBinding); err != nil {
		return fmt.Errorf("failed to create fluent bit cluster role Binding: %w", err)
	}

	exporterMetricsService := makeExporterMetricsService(names.DaemonSet)
	if err := k8sutils.CreateOrUpdateService(ctx, c, exporterMetricsService); err != nil {
		return fmt.Errorf("failed to reconcile exporter metrics service: %w", err)
	}

	metricsService := makeMetricsService(names.DaemonSet)
	if err := k8sutils.CreateOrUpdateService(ctx, c, metricsService); err != nil {
		return fmt.Errorf("failed to reconcile fluent bit metrics service: %w", err)
	}

	cm := makeConfigMap(names.DaemonSet)
	if err := k8sutils.CreateOrUpdateConfigMap(ctx, c, cm); err != nil {
		return fmt.Errorf("failed to reconcile fluent bit configmap: %w", err)
	}

	luaCm := makeLuaConfigMap(names.LuaConfigMap)
	if err := k8sutils.CreateOrUpdateConfigMap(ctx, c, luaCm); err != nil {
		return fmt.Errorf("failed to reconcile fluent bit lua configmap: %w", err)
	}

	parsersCm := makeParserConfigmap(names.ParsersConfigMap)
	if err := k8sutils.CreateIfNotExistsConfigMap(ctx, c, parsersCm); err != nil {
		return fmt.Errorf("failed to reconcile fluent bit parseopts.Config.ap: %w", err)
	}

	checksum, err := calculateChecksum(ctx, c, names)

	if err != nil {
		return fmt.Errorf("failed to calculate config checksum: %w", err)
	}

	daemonSet := aad.makeDaemonSet(names.DaemonSet.Namespace, checksum)
	if err := k8sutils.CreateOrUpdateDaemonSet(ctx, c, daemonSet); err != nil {
		return err
	}

	networkPolicy := commonresources.MakeNetworkPolicy(names.DaemonSet, opts.AllowedPorts, Labels(), selectorLabels())
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
	resourcesFluentBit := corev1.ResourceRequirements{
		Requests: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    aad.cpuRequest,
			corev1.ResourceMemory: aad.memoryRequest,
		},
		Limits: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceMemory: aad.memoryLimit,
		},
	}

	// Set resource requests/limits for directory-size exporter
	resourcesExporter := corev1.ResourceRequirements{
		Requests: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    resource.MustParse("1m"),
			corev1.ResourceMemory: resource.MustParse("5Mi"),
		},
		Limits: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceMemory: resource.MustParse("50Mi"),
		},
	}

	annotations := make(map[string]string)
	annotations[commonresources.AnnotationKeyChecksumConfig] = checksum
	annotations[commonresources.AnnotationKeyIstioExcludeInboundPorts] = fmt.Sprintf("%v,%v", ports.HTTP, ports.ExporterMetrics)

	podLabels := Labels()
	maps.Copy(podLabels, aad.extraPodLabel)

	return &appsv1.DaemonSet{
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
					Labels:      podLabels,
					Annotations: annotations,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: LogAgentName,
					PriorityClassName:  aad.priorityClassName,
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot:   ptr.To(false),
						SeccompProfile: &corev1.SeccompProfile{Type: "RuntimeDefault"},
					},
					Containers: []corev1.Container{
						{
							Name: "fluent-bit",
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: ptr.To(false),
								Capabilities: &corev1.Capabilities{
									Add:  []corev1.Capability{"FOWNER"},
									Drop: []corev1.Capability{"ALL"},
								},
								Privileged:             ptr.To(false),
								ReadOnlyRootFilesystem: ptr.To(true),
							},
							Image:           aad.fluentBitImage,
							ImagePullPolicy: "IfNotPresent",
							EnvFrom: []corev1.EnvFromSource{
								{
									SecretRef: &corev1.SecretEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{Name: fmt.Sprintf("%s-env", LogAgentName)},
										Optional:             ptr.To(true),
									},
								},
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: ports.HTTP,
									Protocol:      "TCP",
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/",
										Port: intstr.FromString("http"),
									},
								},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/api/v1/health",
										Port: intstr.FromString("http"),
									},
								},
							},
							Resources: resourcesFluentBit,
							VolumeMounts: []corev1.VolumeMount{
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
							},
						},
						{
							Name:      "exporter",
							Image:     aad.exporterImage,
							Resources: resourcesExporter,
							Args: []string{
								"--storage-path=/data/flb-storage/",
								"--metric-name=telemetry_fsbuffer_usage_bytes",
							},
							WorkingDir: "",
							Ports: []corev1.ContainerPort{
								{
									Name:          "http-metrics",
									ContainerPort: ports.ExporterMetrics,
									Protocol:      "TCP",
								},
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: ptr.To(false),
								Privileged:               ptr.To(false),
								ReadOnlyRootFilesystem:   ptr.To(true),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "varfluentbit", MountPath: "/data"},
							},
						},
					},
					Volumes: []corev1.Volume{
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
					},
				},
			},
		},
	}
}

func calculateChecksum(ctx context.Context, c client.Client, names ResourceNames) (string, error) {
	var baseCm corev1.ConfigMap
	if err := c.Get(ctx, names.DaemonSet, &baseCm); err != nil {
		return "", fmt.Errorf("failed to get %s/%s ConfigMap: %w", names.DaemonSet.Namespace, names.DaemonSet.Name, err)
	}

	var parsersCm corev1.ConfigMap
	if err := c.Get(ctx, names.ParsersConfigMap, &parsersCm); err != nil {
		return "", fmt.Errorf("failed to get %s/%s ConfigMap: %w", names.ParsersConfigMap.Namespace, names.ParsersConfigMap.Name, err)
	}

	var luaCm corev1.ConfigMap
	if err := c.Get(ctx, names.LuaConfigMap, &luaCm); err != nil {
		return "", fmt.Errorf("failed to get %s/%s ConfigMap: %w", names.LuaConfigMap.Namespace, names.LuaConfigMap.Name, err)
	}

	var sectionsCm corev1.ConfigMap
	if err := c.Get(ctx, names.SectionsConfigMap, &sectionsCm); err != nil {
		return "", fmt.Errorf("failed to get %s/%s ConfigMap: %w", names.SectionsConfigMap.Namespace, names.SectionsConfigMap.Name, err)
	}

	var filesCm corev1.ConfigMap
	if err := c.Get(ctx, names.FilesConfigMap, &filesCm); err != nil {
		return "", fmt.Errorf("failed to get %s/%s ConfigMap: %w", names.FilesConfigMap.Namespace, names.FilesConfigMap.Name, err)
	}

	var envSecret corev1.Secret
	if err := c.Get(ctx, names.EnvConfigSecret, &envSecret); err != nil {
		return "", fmt.Errorf("failed to get %s/%s Secret: %w", names.EnvConfigSecret.Namespace, names.EnvConfigSecret.Name, err)
	}

	var tlsSecret corev1.Secret
	if err := c.Get(ctx, names.TLSFileConfigSecret, &tlsSecret); err != nil {
		return "", fmt.Errorf("failed to get %s/%s Secret: %w", names.TLSFileConfigSecret.Namespace, names.TLSFileConfigSecret.Name, err)
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
