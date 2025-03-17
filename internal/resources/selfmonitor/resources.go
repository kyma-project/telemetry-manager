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
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/configchecksum"
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/ports"
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
	memoryLimit       = resource.MustParse("180Mi")
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
}

func (ad *ApplierDeleter) DeleteResources(ctx context.Context, c client.Client) error {
	objectMeta := metav1.ObjectMeta{
		Name:      ad.Config.BaseName,
		Namespace: ad.Config.Namespace,
	}

	if err := k8sutils.DeleteObject(ctx, c, &appsv1.Deployment{ObjectMeta: objectMeta}); err != nil {
		return err
	}

	if err := k8sutils.DeleteObject(ctx, c, &corev1.ConfigMap{ObjectMeta: objectMeta}); err != nil {
		return err
	}

	if err := k8sutils.DeleteObject(ctx, c, &networkingv1.NetworkPolicy{ObjectMeta: objectMeta}); err != nil {
		return err
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
	// Create RBAC resources in the following order: service account, cluster role, cluster role binding.
	if err := k8sutils.CreateOrUpdateServiceAccount(ctx, c, ad.makeServiceAccount()); err != nil {
		return fmt.Errorf("failed to create self-monitor service account: %w", err)
	}

	if err := k8sutils.CreateOrUpdateRole(ctx, c, ad.makeRole()); err != nil {
		return fmt.Errorf("failed to create self-monitor role: %w", err)
	}

	if err := k8sutils.CreateOrUpdateRoleBinding(ctx, c, ad.makeRoleBinding()); err != nil {
		return fmt.Errorf("failed to create self-monitor role binding: %w", err)
	}

	if err := k8sutils.CreateOrUpdateNetworkPolicy(ctx, c, ad.makeNetworkPolicy()); err != nil {
		return fmt.Errorf("failed to create self-monitor network policy: %w", err)
	}

	configMap := ad.makeConfigMap(opts.PrometheusConfigFileName, opts.PrometheusConfigYAML, opts.AlertRulesFileName, opts.AlertRulesYAML)
	if err := k8sutils.CreateOrUpdateConfigMap(ctx, c, configMap); err != nil {
		return fmt.Errorf("failed to create self-monitor configmap: %w", err)
	}

	checksum := configchecksum.Calculate([]corev1.ConfigMap{*configMap}, nil)
	if err := k8sutils.CreateOrUpdateDeployment(ctx, c, ad.makeDeployment(checksum, opts.PrometheusConfigPath, opts.PrometheusConfigFileName)); err != nil {
		return fmt.Errorf("failed to create sel-monitor deployment: %w", err)
	}

	if err := k8sutils.CreateOrUpdateService(ctx, c, ad.makeService(ports.PrometheusPort)); err != nil {
		return fmt.Errorf("failed to create self-monitor service: %w", err)
	}

	return nil
}

func (ad *ApplierDeleter) makeServiceAccount() *corev1.ServiceAccount {
	serviceAccount := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ad.Config.BaseName,
			Namespace: ad.Config.Namespace,
			Labels:    commonresources.MakeDefaultLabels(ad.Config.BaseName, ad.Config.ComponentType),
		},
	}

	return &serviceAccount
}

func (ad *ApplierDeleter) makeRole() *rbacv1.Role {
	role := rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ad.Config.BaseName,
			Namespace: ad.Config.Namespace,
			Labels:    commonresources.MakeDefaultLabels(ad.Config.BaseName, ad.Config.ComponentType),
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
			Name:      ad.Config.BaseName,
			Namespace: ad.Config.Namespace,
			Labels:    commonresources.MakeDefaultLabels(ad.Config.BaseName, ad.Config.ComponentType),
		},
		Subjects: []rbacv1.Subject{{Name: ad.Config.BaseName, Namespace: ad.Config.Namespace, Kind: rbacv1.ServiceAccountKind}},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     ad.Config.BaseName,
		},
	}

	return &roleBinding
}

func (ad *ApplierDeleter) makeNetworkPolicy() *networkingv1.NetworkPolicy {
	allowedPorts := []int32{ports.PrometheusPort}

	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ad.Config.BaseName,
			Namespace: ad.Config.Namespace,
			Labels:    commonresources.MakeDefaultLabels(ad.Config.BaseName, ad.Config.ComponentType),
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: commonresources.MakeDefaultSelectorLabels(ad.Config.BaseName),
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress,
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							IPBlock: &networkingv1.IPBlock{CIDR: "0.0.0.0/0"},
						},
						{
							IPBlock: &networkingv1.IPBlock{CIDR: "::/0"},
						},
					},
					Ports: ad.makeNetworkPolicyPorts(allowedPorts),
				},
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				{
					To: []networkingv1.NetworkPolicyPeer{
						{
							IPBlock: &networkingv1.IPBlock{CIDR: "0.0.0.0/0"},
						},
						{
							IPBlock: &networkingv1.IPBlock{CIDR: "::/0"},
						},
					},
				},
			},
		},
	}
}

func (ad *ApplierDeleter) makeNetworkPolicyPorts(ports []int32) []networkingv1.NetworkPolicyPort {
	var networkPolicyPorts []networkingv1.NetworkPolicyPort

	tcpProtocol := corev1.ProtocolTCP

	for idx := range ports {
		port := intstr.FromInt32(ports[idx])
		networkPolicyPorts = append(networkPolicyPorts, networkingv1.NetworkPolicyPort{
			Protocol: &tcpProtocol,
			Port:     &port,
		})
	}

	return networkPolicyPorts
}

func (ad *ApplierDeleter) makeConfigMap(prometheusConfigFileName, prometheusConfigYAML, alertRulesFileName, alertRulesYAML string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ad.Config.BaseName,
			Namespace: ad.Config.Namespace,
			Labels:    commonresources.MakeDefaultLabels(ad.Config.BaseName, ad.Config.ComponentType),
		},
		Data: map[string]string{
			prometheusConfigFileName: prometheusConfigYAML,
			alertRulesFileName:       alertRulesYAML,
		},
	}
}

func (ad *ApplierDeleter) makeDeployment(configChecksum, configPath, configFile string) *appsv1.Deployment {
	var replicas int32 = 1

	labels := commonresources.MakeDefaultLabels(ad.Config.BaseName, ad.Config.ComponentType)
	selectorLabels := commonresources.MakeDefaultSelectorLabels(ad.Config.BaseName)
	podLabels := make(map[string]string)
	maps.Copy(podLabels, labels)
	podLabels[commonresources.LabelKeyIstioInject] = "false"

	annotations := map[string]string{commonresources.AnnotationKeyChecksumConfig: configChecksum}
	resources := makeResourceRequirements()
	podSpec := makePodSpec(ad.Config.BaseName, ad.Config.Deployment.Image, configPath, configFile,
		commonresources.WithPriorityClass(ad.Config.Deployment.PriorityClassName),
		commonresources.WithResources(resources),
		commonresources.WithGoMemLimitEnvVar(memoryLimit),
	)

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ad.Config.BaseName,
			Namespace: ad.Config.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(replicas),
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

func makePodSpec(baseName, image, configPath, configFile string, opts ...commonresources.PodSpecOption) corev1.PodSpec {
	var defaultMode int32 = 420

	var prometheusUser int64 = 10001

	var containerName = "self-monitor"
	pod := corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:  containerName,
				Image: image,
				Args: []string{
					"--storage.tsdb.retention.time=" + retentionTime,
					"--storage.tsdb.retention.size=" + retentionSize,
					"--config.file=" + configPath + configFile,
					"--storage.tsdb.path=" + storagePath,
					"--log.format=" + logFormat,
				},
				SecurityContext: &corev1.SecurityContext{
					Privileged:               ptr.To(false),
					RunAsUser:                ptr.To(prometheusUser),
					RunAsNonRoot:             ptr.To(true),
					ReadOnlyRootFilesystem:   ptr.To(true),
					AllowPrivilegeEscalation: ptr.To(false),
					SeccompProfile: &corev1.SeccompProfile{
						Type: corev1.SeccompProfileTypeRuntimeDefault,
					},
					Capabilities: &corev1.Capabilities{
						Drop: []corev1.Capability{"ALL"},
					},
				},
				VolumeMounts: []corev1.VolumeMount{{Name: configFileMountName, MountPath: configPath}, {Name: storageMountName, MountPath: storagePath}},
				Ports: []corev1.ContainerPort{
					{
						Name:          "http-web",
						ContainerPort: ports.PrometheusPort,
					},
				},
				LivenessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{Path: "/-/healthy", Port: intstr.IntOrString{IntVal: ports.PrometheusPort}},
					},
					FailureThreshold: 5, //nolint:mnd // 5 failures
					PeriodSeconds:    5, //nolint:mnd // 5 seconds
					TimeoutSeconds:   3, //nolint:mnd // 3 seconds
					SuccessThreshold: 1, //nolint:mnd // 1 success
				},
				ReadinessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{Path: "/-/ready", Port: intstr.IntOrString{IntVal: ports.PrometheusPort}},
					},
					FailureThreshold: 3, //nolint:mnd // 3 failures
					PeriodSeconds:    5, //nolint:mnd // 5 seconds
					TimeoutSeconds:   3, //nolint:mnd // 3 seconds
					SuccessThreshold: 1, //nolint:mnd // 1 success
				},
			},
		},
		ServiceAccountName:            baseName,
		TerminationGracePeriodSeconds: ptr.To(int64(300)), //nolint:mnd // 300 seconds
		SecurityContext: &corev1.PodSecurityContext{
			RunAsUser:    ptr.To(prometheusUser),
			RunAsNonRoot: ptr.To(true),
			SeccompProfile: &corev1.SeccompProfile{
				Type: corev1.SeccompProfileTypeRuntimeDefault,
			},
		},
		Volumes: []corev1.Volume{
			{
				Name: configFileMountName,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						DefaultMode: &defaultMode,
						LocalObjectReference: corev1.LocalObjectReference{
							Name: baseName,
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
		},
	}

	for _, opt := range opts {
		opt(&pod)
	}

	return pod
}

func makeResourceRequirements() corev1.ResourceRequirements {
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

func (ad *ApplierDeleter) makeService(port int32) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ad.Config.BaseName,
			Namespace: ad.Config.Namespace,
			Labels:    commonresources.MakeDefaultLabels(ad.Config.BaseName, ad.Config.ComponentType),
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
			Selector: commonresources.MakeDefaultSelectorLabels(ad.Config.BaseName),
			Type:     corev1.ServiceTypeClusterIP,
		},
	}
}
