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

	"github.com/kyma-project/telemetry-manager/internal/config"
	"github.com/kyma-project/telemetry-manager/internal/configchecksum"
	"github.com/kyma-project/telemetry-manager/internal/fluentbit/config/builder"
	fbports "github.com/kyma-project/telemetry-manager/internal/fluentbit/ports"
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

	exporterContainerName  = "exporter"
	chownInitContainerName = "checkpoint-dir-ownership-modifier"

	// Volume names
	configVolumeName                = "config"
	luaScriptsVolumeName            = "luascripts"
	varLogVolumeName                = "varlog"
	sharedFluentBitConfigVolumeName = "shared-fluent-bit-config"
	dynamicConfigVolumeName         = "dynamic-config"
	dynamicParsersConfigVolumeName  = "dynamic-parsers-config"
	dynamicFilesVolumeName          = "dynamic-files"
	varFluentBitVolumeName          = "varfluentbit"
	outputTLSConfigVolumeName       = "output-tls-config"

	// Volume mount paths
	configVolumeFluentBitMountPath       = "/fluent-bit/etc/fluent-bit.conf"
	configVolumeCustomParsersMountPath   = "/fluent-bit/etc/custom_parsers.conf"
	luaScriptsVolumeMountPath            = "/fluent-bit/scripts/filter-script.lua"
	varLogVolumeMountPath                = "/var/log"
	sharedFluentBitConfigVolumeMountPath = "/fluent-bit/etc"
	dynamicConfigVolumeMountPath         = "/fluent-bit/etc/dynamic/"
	dynamicParsersConfigVolumeMountPath  = "/fluent-bit/etc/dynamic-parsers/"
	dynamicFilesVolumeMountPath          = "/files"
	varFluentBitVolumeMountPath          = "/data"
	outputTLSConfigVolumeMountPath       = "/fluent-bit/etc/output-tls-config/"
)

var (
	fbContainerCPURequest    = resource.MustParse("100m")
	fbContainerMemoryRequest = resource.MustParse("50Mi")
	fbContainerMemoryLimit   = resource.MustParse("1Gi")

	exporterContainerCPURequest    = resource.MustParse("1m")
	exporterContainerMemoryRequest = resource.MustParse("5Mi")
	exporterContainerMemoryLimit   = resource.MustParse("50Mi")
)

// AgentApplyOptions expects a syncerClient which is a client with no ownerReference setter since it handles its
// own resource deletion with finalizers and will be removed once the ConfigBuilder implementation is done.
type AgentApplyOptions struct {
	AllowedPorts    []int32
	FluentBitConfig *builder.FluentBitConfig
}

type AgentApplierDeleter struct {
	extraPodLabels          map[string]string
	fluentBitImage          string
	exporterImage           string
	chownInitContainerImage string
	priorityClassName       string
	namespace               string

	daemonSetName           types.NamespacedName
	luaConfigMapName        types.NamespacedName
	parsersConfigMapName    types.NamespacedName
	filesConfigMapName      types.NamespacedName
	sectionsConfigMapName   types.NamespacedName
	envConfigSecretName     types.NamespacedName
	tlsFileConfigSecretName types.NamespacedName
	globals                 config.Global
}

func NewFluentBitApplierDeleter(global config.Global, namespace, fbImage, exporterImage, chownInitContainerImage, priorityClassName string) *AgentApplierDeleter {
	return &AgentApplierDeleter{
		globals:   global,
		namespace: namespace,
		extraPodLabels: map[string]string{
			commonresources.LabelKeyIstioInject:        commonresources.LabelValueTrue,
			commonresources.LabelKeyTelemetryLogExport: commonresources.LabelValueTrue,
		},
		fluentBitImage:          fbImage,
		exporterImage:           exporterImage,
		chownInitContainerImage: chownInitContainerImage,
		priorityClassName:       priorityClassName,

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

	checksum := configchecksum.Calculate([]corev1.ConfigMap{*cm, *luaCm, *sectionsCm, *filesCm}, []corev1.Secret{*envConfigSecret, *tlsFileConfigSecret})

	daemonSet := aad.makeDaemonSet(aad.daemonSetName.Namespace, checksum)
	if err := k8sutils.CreateOrUpdateDaemonSet(ctx, c, daemonSet); err != nil {
		return err
	}

	networkPolicy := commonresources.MakeNetworkPolicy(aad.daemonSetName, opts.AllowedPorts, makeLabels(), selectorLabels())
	if err := k8sutils.CreateOrUpdateNetworkPolicy(ctx, c, networkPolicy); err != nil {
		return fmt.Errorf("failed to create fluent bit network policy: %w", err)
	}

	//TODO: remove parser configmap creation after logparser removal is rolled out
	parserCm := corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name:      fbParsersConfigMapName,
		Namespace: aad.namespace,
	}}
	if err := k8sutils.DeleteObject(ctx, c, &parserCm); err != nil {
		return fmt.Errorf("failed to delete parser configmap: %w", err)
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
	annotations := make(map[string]string)
	annotations[commonresources.AnnotationKeyChecksumConfig] = checksum
	annotations[commonresources.AnnotationKeyIstioExcludeInboundPorts] = fmt.Sprintf("%v,%v", fbports.HTTP, fbports.ExporterMetrics)

	// Create final annotations for the DaemonSet and Pods with additional annotations
	podAnnotations := make(map[string]string)
	resourceAnnotations := make(map[string]string)

	// Copy global additional annotations
	maps.Copy(resourceAnnotations, aad.globals.AdditionalAnnotations())
	maps.Copy(podAnnotations, aad.globals.AdditionalAnnotations())
	maps.Copy(podAnnotations, annotations)

	defaultPodLabels := makeLabels()
	maps.Copy(defaultPodLabels, aad.extraPodLabels)

	// Create final labels for the DaemonSet and Pods with additional labels
	resourceLabels := make(map[string]string)
	podLabels := make(map[string]string)

	maps.Copy(resourceLabels, aad.globals.AdditionalLabels())
	maps.Copy(podLabels, aad.globals.AdditionalLabels())
	maps.Copy(resourceLabels, makeLabels())
	maps.Copy(podLabels, defaultPodLabels)

	fluentBitResources := commonresources.MakeResourceRequirements(
		fbContainerMemoryLimit,
		fbContainerMemoryRequest,
		fbContainerCPURequest,
	)

	exporterResources := commonresources.MakeResourceRequirements(
		exporterContainerMemoryLimit,
		exporterContainerMemoryRequest,
		exporterContainerCPURequest,
	)

	ds := &appsv1.DaemonSet{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:        LogAgentName,
			Namespace:   namespace,
			Labels:      resourceLabels,
			Annotations: resourceAnnotations,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels(),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: podAnnotations,
					Labels:      podLabels,
				},
				Spec: commonresources.MakePodSpec(LogAgentName,
					commonresources.WithPriorityClass(aad.priorityClassName),
					commonresources.WithTolerations(commonresources.CriticalDaemonSetTolerations),
					commonresources.WithVolumes(aad.fluentBitVolumes()),
					commonresources.WithClusterTrustBundleVolume(aad.globals.ClusterTrustBundleName()),
					commonresources.WithImagePullSecretName(aad.globals.ImagePullSecretName()),
					commonresources.WithContainer("fluent-bit", aad.fluentBitImage,
						commonresources.WithEnvVarsFromSecret(fmt.Sprintf("%s-env", LogAgentName)),
						commonresources.WithRunAsGroup(commonresources.GroupRoot),
						commonresources.WithRunAsUser(commonresources.UserDefault),
						commonresources.WithPort("http", fbports.HTTP),
						commonresources.WithProbes(aad.fluentBitLivenessProbe(), aad.fluentBitReadinessProbe()),
						commonresources.WithResources(fluentBitResources),
						commonresources.WithVolumeMounts(aad.fluentBitVolumeMounts()),
						commonresources.WithClusterTrustBundleVolumeMount(aad.globals.ClusterTrustBundleName()),
					),
					commonresources.WithContainer(exporterContainerName, aad.exporterImage,
						commonresources.WithArgs([]string{
							"--storage-path=/data/flb-storage/",
							"--metric-name=telemetry_fsbuffer_usage_bytes",
						}),
						commonresources.WithPort("http-metrics", fbports.ExporterMetrics),
						commonresources.WithResources(exporterResources),
						commonresources.WithVolumeMounts(aad.exporterVolumeMounts()),
					),
					// init container for changing the owner of the storage volume to be fluentbit
					commonresources.WithInitContainer(chownInitContainerName, aad.chownInitContainerImage,
						commonresources.WithChownInitContainerOpts(varFluentBitVolumeMountPath, aad.chownInitContainerVolumeMounts())...,
					),
				),
			},
		},
	}

	return ds
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
		{MountPath: sharedFluentBitConfigVolumeMountPath, Name: sharedFluentBitConfigVolumeName},
		{MountPath: configVolumeFluentBitMountPath, Name: configVolumeName, SubPath: "fluent-bit.conf"},
		{MountPath: dynamicConfigVolumeMountPath, Name: dynamicConfigVolumeName},
		{MountPath: dynamicParsersConfigVolumeMountPath, Name: dynamicParsersConfigVolumeName},
		{MountPath: configVolumeCustomParsersMountPath, Name: configVolumeName, SubPath: "custom_parsers.conf"},
		{MountPath: luaScriptsVolumeMountPath, Name: luaScriptsVolumeName, SubPath: "filter-script.lua"},
		{MountPath: varLogVolumeMountPath, Name: varLogVolumeName, ReadOnly: true},
		{MountPath: varFluentBitVolumeMountPath, Name: varFluentBitVolumeName},
		{MountPath: dynamicFilesVolumeMountPath, Name: dynamicFilesVolumeName},
		{MountPath: outputTLSConfigVolumeMountPath, Name: outputTLSConfigVolumeName, ReadOnly: true},
	}
}

func (aad *AgentApplierDeleter) exporterVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{MountPath: varFluentBitVolumeMountPath, Name: varFluentBitVolumeName},
	}
}

func (aad *AgentApplierDeleter) chownInitContainerVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{MountPath: varFluentBitVolumeMountPath, Name: varFluentBitVolumeName},
	}
}

func (aad *AgentApplierDeleter) fluentBitVolumes() []corev1.Volume {
	return []corev1.Volume{
		{
			Name: configVolumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: LogAgentName},
				},
			},
		},
		{
			Name: luaScriptsVolumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: fmt.Sprintf("%s-luascripts", LogAgentName)},
				},
			},
		},
		{
			Name: varLogVolumeName,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{Path: "/var/log"},
			},
		},
		{
			Name: sharedFluentBitConfigVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: dynamicConfigVolumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: fmt.Sprintf("%s-sections", LogAgentName)},
					Optional:             ptr.To(true),
				},
			},
		},
		{
			Name: dynamicParsersConfigVolumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: fmt.Sprintf("%s-parsers", LogAgentName)},
					Optional:             ptr.To(true),
				},
			},
		},
		{
			Name: dynamicFilesVolumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: fmt.Sprintf("%s-files", LogAgentName)},
					Optional:             ptr.To(true),
				},
			},
		},
		{
			Name: varFluentBitVolumeName,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{Path: fmt.Sprintf("/var/%s", LogAgentName)},
			},
		},
		{
			Name: outputTLSConfigVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: fmt.Sprintf("%s-output-tls-config", LogAgentName),
				},
			},
		},
	}
}

func makeClusterRole(name types.NamespacedName) *rbacv1.ClusterRole {
	clusterRole := rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
			Labels:    makeLabels(),
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
	serviceLabels := makeLabels()
	serviceLabels[commonresources.LabelKeyTelemetrySelfMonitor] = commonresources.LabelValueTelemetrySelfMonitor

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-metrics", name.Name),
			Namespace: name.Namespace,
			Labels:    serviceLabels,
			Annotations: map[string]string{
				commonresources.AnnotationKeyPrometheusScrape: "true",
				commonresources.AnnotationKeyPrometheusPort:   strconv.Itoa(fbports.HTTP),
				commonresources.AnnotationKeyPrometheusScheme: "http",
				commonresources.AnnotationKeyPrometheusPath:   "/api/v2/metrics/prometheus",
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Protocol:   "TCP",
					Port:       int32(fbports.HTTP),
					TargetPort: intstr.FromString("http"),
				},
			},
			Selector: selectorLabels(),
			Type:     corev1.ServiceTypeClusterIP,
		},
	}
}

func makeExporterMetricsService(name types.NamespacedName) *corev1.Service {
	serviceLabels := makeLabels()

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-exporter-metrics", name.Name),
			Namespace: name.Namespace,
			Labels:    serviceLabels,
			Annotations: map[string]string{
				commonresources.AnnotationKeyPrometheusScrape: "true",
				commonresources.AnnotationKeyPrometheusPort:   strconv.Itoa(fbports.ExporterMetrics),
				commonresources.AnnotationKeyPrometheusScheme: "http",
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "http-metrics",
					Protocol:   "TCP",
					Port:       int32(fbports.ExporterMetrics),
					TargetPort: intstr.FromString("http-metrics"),
				},
			},
			Selector: selectorLabels(),
			Type:     corev1.ServiceTypeClusterIP,
		},
	}
}

func makeConfigMap(name types.NamespacedName) *corev1.ConfigMap {
	fluentBitConfig := `
[SERVICE]
    Daemon Off
    Flush 1
    Log_Level warn
    HTTP_Server On
    HTTP_Listen 0.0.0.0
    HTTP_Port {{ HTTP_PORT }}
    storage.path /data/flb-storage/
    storage.metrics on

@INCLUDE dynamic/*.conf
`
	fluentBitConfig = strings.Replace(fluentBitConfig, "{{ HTTP_PORT }}", strconv.Itoa(fbports.HTTP), 1)

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
			Labels:    makeLabels(),
		},
		Data: map[string]string{
			"fluent-bit.conf": fluentBitConfig,
		},
	}
}

func makeSectionsConfigMap(name types.NamespacedName, sectionsConfig map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
			Labels:    makeLabels(),
		},
		Data: sectionsConfig,
	}
}

func makeFilesConfigMap(name types.NamespacedName, filesConfig map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
			Labels:    makeLabels(),
		},
		Data: filesConfig,
	}
}

func makeLuaConfigMap(name types.NamespacedName) *corev1.ConfigMap {
	//nolint:dupword // Ignore lua syntax code duplications.
	luaFilter := `
function enrich_app_name(tag, timestamp, record)
  if record.kubernetes == nil then
    return 0
  end
  enrich_app_name_internal(record.kubernetes)
  return 2, timestamp, record
end
function dedot_and_enrich_app_name(tag, timestamp, record)
  if record.kubernetes == nil then
    return 0
  end
  enrich_app_name_internal(record.kubernetes)
  map_keys(record.kubernetes.annotations)
  map_keys(record.kubernetes.labels)
  return 2, timestamp, record
end
function enrich_app_name_internal(table)
  if table.labels == nil then
    return 0
  end
  table["app_name"] = table.labels["app.kubernetes.io/name"] or table.labels["app"]
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
			Labels:    makeLabels(),
		},
		Data: map[string]string{"filter-script.lua": luaFilter},
	}
}

func makeEnvConfigSecret(name types.NamespacedName, envConfigSecret map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
			Labels:    makeLabels(),
		},
		Data: envConfigSecret,
	}
}

func makeTLSFileConfigSecret(name types.NamespacedName, tlsFileConfigSecret map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
			Labels:    makeLabels(),
		},
		Data: tlsFileConfigSecret,
	}
}

func makeLabels() map[string]string {
	result := commonresources.MakeDefaultLabels("fluent-bit", commonresources.LabelValueK8sComponentAgent)
	result[commonresources.LabelKeyK8sInstance] = commonresources.LabelValueK8sInstance

	return result
}

func selectorLabels() map[string]string {
	result := commonresources.MakeDefaultSelectorLabels("fluent-bit")
	result[commonresources.LabelKeyK8sInstance] = commonresources.LabelValueK8sInstance

	return result
}
