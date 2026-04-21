package selfmonitor

import (
	"context"
	"fmt"
	"maps"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/configchecksum"
	fbports "github.com/kyma-project/telemetry-manager/internal/fluentbit/ports"
	"github.com/kyma-project/telemetry-manager/internal/k8sclients"
	mgrports "github.com/kyma-project/telemetry-manager/internal/manager/ports"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	"github.com/kyma-project/telemetry-manager/internal/resources/names"
	selfmonports "github.com/kyma-project/telemetry-manager/internal/selfmonitor/ports"
	k8sutils "github.com/kyma-project/telemetry-manager/internal/utils/k8s"
)

const (
	retentionTime       = "2h"
	retentionSize       = "50MB"
	logFormat           = "json"
	configFileMountName = "prometheus-config-volume"
	storageMountName    = "prometheus-storage-volume"
	storagePath         = "/prometheus/"
)

var (
	storageVolumeSize = resource.MustParse("1000Mi")
	cpuRequest        = resource.MustParse("10m")
	memoryRequest     = resource.MustParse("50Mi")
	memoryLimit       = resource.MustParse("512Mi")
)

type ApplierDeleter struct {
	Config Config
}

type ApplyOptions struct {
	AlertRulesFileName       string
	AlertRulesYAML           string
	PrometheusConfigFileName string
	PrometheusConfigPath     string
	PrometheusConfigYAML     string
	LogLevel                 string
}

func (ad *ApplierDeleter) DeleteResources(ctx context.Context, c client.Client) error {
	objectMeta := metav1.ObjectMeta{
		Name:      names.SelfMonitor,
		Namespace: ad.Config.TargetNamespace(),
	}

	if err := k8sutils.DeleteObject(ctx, c, &appsv1.Deployment{ObjectMeta: objectMeta}); err != nil {
		return err
	}

	if err := k8sutils.DeleteObject(ctx, c, &corev1.ConfigMap{ObjectMeta: objectMeta}); err != nil {
		return err
	}

	networkPolicySelector := map[string]string{
		commonresources.LabelKeyK8sName: names.SelfMonitor,
	}

	if err := k8sutils.DeleteObjectsByLabelSelector(ctx, c, &networkingv1.NetworkPolicyList{}, networkPolicySelector); err != nil {
		return fmt.Errorf("failed to delete network policy: %w", err)
	}

	if err := k8sutils.DeleteObject(ctx, c, &rbacv1.RoleBinding{ObjectMeta: objectMeta}); err != nil {
		return err
	}

	if err := k8sutils.DeleteObject(ctx, c, &rbacv1.Role{ObjectMeta: objectMeta}); err != nil {
		return err
	}

	if err := k8sutils.DeleteObject(ctx, c, &corev1.ServiceAccount{ObjectMeta: objectMeta}); err != nil {
		return err
	}

	if err := k8sutils.DeleteObject(ctx, c, &corev1.Service{ObjectMeta: objectMeta}); err != nil {
		return err
	}

	return nil
}

func (ad *ApplierDeleter) ApplyResources(ctx context.Context, c client.Client, opts ApplyOptions) error {
	labelerClient := k8sclients.NewLabeler(c, commonresources.DefaultLabels(names.SelfMonitor, commonresources.LabelValueK8sComponentMonitor))

	// Create RBAC resources in the following order: service account, cluster role, cluster role binding.
	if err := k8sutils.CreateOrUpdateServiceAccount(ctx, labelerClient, ad.makeServiceAccount()); err != nil {
		return fmt.Errorf("failed to create self-monitor service account: %w", err)
	}

	if err := k8sutils.CreateOrUpdateRole(ctx, labelerClient, ad.makeRole()); err != nil {
		return fmt.Errorf("failed to create self-monitor role: %w", err)
	}

	if err := k8sutils.CreateOrUpdateRoleBinding(ctx, labelerClient, ad.makeRoleBinding()); err != nil {
		return fmt.Errorf("failed to create self-monitor role binding: %w", err)
	}

	networkPolicies := ad.makeNetworkPolicies()

	for _, np := range networkPolicies {
		if err := k8sutils.CreateOrUpdateNetworkPolicy(ctx, labelerClient, np); err != nil {
			return fmt.Errorf("failed to create self monitor network policies: %w", err)
		}
	}

	configMap := ad.makeConfigMap(opts.PrometheusConfigFileName, opts.PrometheusConfigYAML, opts.AlertRulesFileName, opts.AlertRulesYAML)
	if err := k8sutils.CreateOrUpdateConfigMap(ctx, labelerClient, configMap); err != nil {
		return fmt.Errorf("failed to create self-monitor configmap: %w", err)
	}

	checksum := configchecksum.Calculate([]corev1.ConfigMap{*configMap}, nil)
	if err := k8sutils.CreateOrUpdateDeployment(ctx, labelerClient, ad.makeDeployment(checksum, opts.PrometheusConfigPath, opts.PrometheusConfigFileName, opts.LogLevel)); err != nil {
		return fmt.Errorf("failed to create sel-monitor deployment: %w", err)
	}

	if err := k8sutils.CreateOrUpdateService(ctx, labelerClient, ad.makeService(selfmonports.PrometheusPort)); err != nil {
		return fmt.Errorf("failed to create self-monitor service: %w", err)
	}

	// TODO: Remove after rollout 1.59.0
	if err := commonresources.CleanupOldNetworkPolicy(ctx, labelerClient, ad.selfMonitorName()); err != nil {
		return fmt.Errorf("failed to cleanup old network policy: %w", err)
	}

	return nil
}

func (ad *ApplierDeleter) makeServiceAccount() *corev1.ServiceAccount {
	serviceAccount := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.SelfMonitor,
			Namespace: ad.Config.TargetNamespace(),
		},
	}

	return &serviceAccount
}

func (ad *ApplierDeleter) makeRole() *rbacv1.Role {
	role := rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.SelfMonitor,
			Namespace: ad.Config.TargetNamespace(),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"services", "endpoints", "pods"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	}

	return &role
}

func (ad *ApplierDeleter) makeRoleBinding() *rbacv1.RoleBinding {
	roleBinding := rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.SelfMonitor,
			Namespace: ad.Config.TargetNamespace(),
		},
		Subjects: []rbacv1.Subject{{Name: names.SelfMonitor, Namespace: ad.Config.TargetNamespace(), Kind: rbacv1.ServiceAccountKind}},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     names.SelfMonitor,
		},
	}

	return &roleBinding
}

func (ad *ApplierDeleter) makeConfigMap(prometheusConfigFileName, prometheusConfigYAML, alertRulesFileName, alertRulesYAML string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.SelfMonitor,
			Namespace: ad.Config.TargetNamespace(),
		},
		Data: map[string]string{
			prometheusConfigFileName: prometheusConfigYAML,
			alertRulesFileName:       alertRulesYAML,
		},
	}
}

func (ad *ApplierDeleter) makeDeployment(configChecksum, configPath, configFile, logLevel string) *appsv1.Deployment {
	var replicas int32 = 1

	// Resource labels: only additional labels from globals; default labels are applied by the labeler
	resourceLabels := make(map[string]string)
	maps.Copy(resourceLabels, ad.Config.AdditionalWorkloadLabels())

	// Pod labels: need default labels explicitly since the labeler only sets top-level object labels
	podLabels := make(map[string]string)
	maps.Copy(podLabels, commonresources.DefaultLabels(names.SelfMonitor, commonresources.LabelValueK8sComponentMonitor))
	maps.Copy(podLabels, ad.Config.AdditionalWorkloadPodLabels())
	podLabels[commonresources.LabelKeyIstioInject] = commonresources.LabelValueFalse
	podLabels[commonresources.LabelKeyTelemetryMetricsScraping] = commonresources.LabelValueTelemetryMetricsScraping

	// Resource annotations: only additional annotations from globals
	resourceAnnotations := make(map[string]string)
	maps.Copy(resourceAnnotations, ad.Config.AdditionalWorkloadAnnotations())

	// Pod annotations: only pod-specific annotations plus config checksum
	podAnnotations := make(map[string]string)
	maps.Copy(podAnnotations, ad.Config.AdditionalWorkloadPodAnnotations())
	podAnnotations[commonresources.AnnotationKeyChecksumConfig] = configChecksum

	podSpec := ad.makePodSpec(ad.Config.Image, configPath, configFile, logLevel)

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        names.SelfMonitor,
			Namespace:   ad.Config.TargetNamespace(),
			Labels:      resourceLabels,
			Annotations: resourceAnnotations,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: new(replicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: commonresources.DefaultSelector(names.SelfMonitor),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      podLabels,
					Annotations: podAnnotations,
				},
				Spec: podSpec,
			},
		},
	}
}

func (ad *ApplierDeleter) makePodSpec(image, configPath, configFile, logLevel string) corev1.PodSpec {
	var defaultMode int32 = 420

	if logLevel == "" {
		logLevel = "warn"
	}

	args := []string{
		"--storage.tsdb.retention.time=" + retentionTime,
		"--storage.tsdb.retention.size=" + retentionSize,
		"--config.file=" + configPath + configFile,
		"--storage.tsdb.path=" + storagePath,
		"--log.format=" + logFormat,
		"--log.level=" + logLevel,
	}

	volumes := []corev1.Volume{
		{
			Name: configFileMountName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					DefaultMode: &defaultMode,
					LocalObjectReference: corev1.LocalObjectReference{
						Name: names.SelfMonitor,
					},
				},
			},
		},
		{
			Name: storageMountName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					SizeLimit: &storageVolumeSize,
				},
			},
		},
	}

	volumeMounts := []corev1.VolumeMount{
		{Name: configFileMountName, MountPath: configPath},
		{Name: storageMountName, MountPath: storagePath},
	}

	liveness := &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/-/healthy",
				Port: intstr.IntOrString{IntVal: selfmonports.PrometheusPort},
			},
		},
		FailureThreshold: 5, //nolint:mnd // 5 failures
		PeriodSeconds:    5, //nolint:mnd // 5 failures
		TimeoutSeconds:   3, //nolint:mnd // 5 failures
		SuccessThreshold: 1, //nolint:mnd // 5 failures
	}

	readiness := &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/-/ready",
				Port: intstr.IntOrString{IntVal: selfmonports.PrometheusPort},
			},
		},
		FailureThreshold: 3, //nolint:mnd // 5 failures
		PeriodSeconds:    5, //nolint:mnd // 5 failures
		TimeoutSeconds:   3, //nolint:mnd // 5 failures
		SuccessThreshold: 1, //nolint:mnd // 5 failures
	}

	resources := commonresources.MakeResourceRequirements(
		memoryLimit,
		memoryRequest,
		cpuRequest,
	)

	opts := []commonresources.PodSpecOption{
		commonresources.WithVolumes(volumes),
		commonresources.WithPodRunAsUser(commonresources.UserDefault),
		commonresources.WithPriorityClass(ad.Config.PriorityClassName),
		commonresources.WithTerminationGracePeriodSeconds(300), //nolint:mnd // 300 seconds
		commonresources.WithImagePullSecretName(ad.Config.ImagePullSecretName()),

		commonresources.WithContainer(names.SelfMonitorContainerName, image,
			commonresources.WithArgs(args),
			commonresources.WithPort("http-web", selfmonports.PrometheusPort),
			commonresources.WithVolumeMounts(volumeMounts),
			commonresources.WithProbes(liveness, readiness),
			commonresources.WithResources(resources),
			commonresources.WithRunAsUser(commonresources.UserDefault),
			commonresources.WithGoMemLimitEnvVar(memoryLimit),
		),
	}

	return commonresources.MakePodSpec(names.SelfMonitor, opts...)
}

func (ad *ApplierDeleter) makeService(port int32) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.SelfMonitor,
			Namespace: ad.Config.TargetNamespace(),
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Protocol:   corev1.ProtocolTCP,
					Port:       port,
					TargetPort: intstr.FromInt32(port),
				},
			},
			Selector: commonresources.DefaultSelector(names.SelfMonitor),
			Type:     corev1.ServiceTypeClusterIP,
		},
	}
}

func (ad *ApplierDeleter) makeNetworkPolicies() []*networkingv1.NetworkPolicy {
	selfMonitorNetworkPolicy := commonresources.MakeNetworkPolicy(
		ad.selfMonitorName(),
		commonresources.DefaultSelector(names.SelfMonitor),
		// Allow egress to FluentBit for scraping metrics
		commonresources.WithEgressToPods(map[string]string{
			commonresources.LabelKeyK8sName: "fluent-bit",
		}, []int32{fbports.HTTP}),
		// Allow egress to Metric Agent for scraping metrics
		commonresources.WithEgressToPods(map[string]string{
			commonresources.LabelKeyK8sName: "telemetry-metric-agent",
		}, []int32{ports.Metrics}),
		// Allow egress to Log Agent for scraping metrics
		commonresources.WithEgressToPods(map[string]string{
			commonresources.LabelKeyK8sName: "telemetry-log-agent",
		}, []int32{ports.Metrics}),
		// Allow egress to OTel gateways for scraping metrics
		commonresources.WithEgressToPods(map[string]string{
			commonresources.LabelKeyK8sComponent: commonresources.LabelValueK8sComponentGateway,
		}, []int32{ports.Metrics}),
		// Allow egress to telemetry-manager for webhook
		commonresources.WithEgressToPods(map[string]string{
			commonresources.LabelKeyK8sName: "manager",
		}, []int32{mgrports.Webhook}),
	)
	metricsNetworkPolicy := commonresources.MakeNetworkPolicy(
		ad.selfMonitorName(),
		commonresources.DefaultSelector(names.SelfMonitor),
		commonresources.WithNameSuffix("metrics"),
		// Allow ingress from telemetry-manager pods only on Prometheus port
		commonresources.WithIngressFromPods(map[string]string{
			commonresources.LabelKeyK8sName: "manager",
		}, []int32{selfmonports.PrometheusPort}),
		// Allow ingress from pods with metrics-scraping label
		commonresources.WithIngressFromPodsInAllNamespaces(map[string]string{
			commonresources.LabelKeyTelemetryMetricsScraping: commonresources.LabelValueTelemetryMetricsScraping,
		}, []int32{selfmonports.PrometheusPort}),
	)

	return []*networkingv1.NetworkPolicy{selfMonitorNetworkPolicy, metricsNetworkPolicy}
}

func (ad *ApplierDeleter) selfMonitorName() types.NamespacedName {
	return types.NamespacedName{
		Name:      names.SelfMonitor,
		Namespace: ad.Config.TargetNamespace(),
	}
}
