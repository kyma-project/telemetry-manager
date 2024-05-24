package selfmonitor

import (
	"context"
	"fmt"
	"maps"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/configchecksum"
	"github.com/kyma-project/telemetry-manager/internal/k8sutils"
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/ports"
)

const (
	retentionTime = "2h"
	retentionSize = "80MB"
)

var (
	storageVolumeSize = resource.MustParse("100Mi")
	cpuRequest        = resource.MustParse("0.1")
	memoryRequest     = resource.MustParse("50Mi")
	cpuLimit          = resource.MustParse("0.2")
	memoryLimit       = resource.MustParse("180Mi")
)

type ApplierDeleter struct {
	Config *Config
}

func (ad *ApplierDeleter) RemoveResources(ctx context.Context, c client.Client) error {
	objectMeta := metav1.ObjectMeta{
		Name:      ad.Config.BaseName,
		Namespace: ad.Config.Namespace,
	}

	if err := deleteObj(ctx, c, &appsv1.Deployment{ObjectMeta: objectMeta}); err != nil {
		return err
	}

	if err := deleteObj(ctx, c, &corev1.ConfigMap{ObjectMeta: objectMeta}); err != nil {
		return err
	}

	if err := deleteObj(ctx, c, &networkingv1.NetworkPolicy{ObjectMeta: objectMeta}); err != nil {
		return err
	}

	if err := deleteObj(ctx, c, &rbacv1.RoleBinding{ObjectMeta: objectMeta}); err != nil {
		return err
	}

	if err := deleteObj(ctx, c, &rbacv1.Role{ObjectMeta: objectMeta}); err != nil {
		return err
	}

	if err := deleteObj(ctx, c, &corev1.ServiceAccount{ObjectMeta: objectMeta}); err != nil {
		return err
	}

	if err := deleteObj(ctx, c, &corev1.Service{ObjectMeta: objectMeta}); err != nil {
		return err
	}

	return nil
}

func deleteObj(ctx context.Context, c client.Client, object client.Object) error {
	if err := c.Delete(ctx, object); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func (ad *ApplierDeleter) ApplyResources(ctx context.Context, c client.Client, prometheusConfigYAML, alertRulesYAML string) error {
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

	configMap := ad.makeConfigMap(prometheusConfigYAML, alertRulesYAML)
	if err := k8sutils.CreateOrUpdateConfigMap(ctx, c, configMap); err != nil {
		return fmt.Errorf("failed to create self-monitor configmap: %w", err)
	}

	checksum := configchecksum.Calculate([]corev1.ConfigMap{*configMap}, nil)
	if err := k8sutils.CreateOrUpdateDeployment(ctx, c, ad.makeDeployment(checksum)); err != nil {
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
			Labels:    ad.defaultLabels(),
		},
	}
	return &serviceAccount
}

func (ad *ApplierDeleter) makeRole() *rbacv1.Role {
	role := rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ad.Config.BaseName,
			Namespace: ad.Config.Namespace,
			Labels:    ad.defaultLabels(),
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
			Labels:    ad.defaultLabels(),
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
	allowedPorts := []int32{int32(ports.PrometheusPort)}
	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ad.Config.BaseName,
			Namespace: ad.Config.Namespace,
			Labels:    ad.defaultLabels(),
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: ad.defaultLabels(),
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

func (ad *ApplierDeleter) makeConfigMap(prometheusConfigYAML, alertRulesYAML string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ad.Config.BaseName,
			Namespace: ad.Config.Namespace,
			Labels:    ad.defaultLabels(),
		},
		Data: map[string]string{
			"prometheus.yml":     prometheusConfigYAML,
			"alerting_rules.yml": alertRulesYAML,
		},
	}
}
func (ad *ApplierDeleter) makeDeployment(configChecksum string) *appsv1.Deployment {
	var replicas int32 = 1
	selectorLabels := ad.defaultLabels()
	podLabels := maps.Clone(selectorLabels)
	podLabels["sidecar.istio.io/inject"] = "false"

	annotations := map[string]string{"checksum/Config": configChecksum}
	resources := makeResourceRequirements()
	podSpec := makePodSpec(ad.Config.BaseName, ad.Config.Deployment.Image,
		commonresources.WithPriorityClass(ad.Config.Deployment.PriorityClassName),
		commonresources.WithResources(resources),
		commonresources.WithGoMemLimitEnvVar(memoryLimit),
	)

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ad.Config.BaseName,
			Namespace: ad.Config.Namespace,
			Labels:    selectorLabels,
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

func (ad *ApplierDeleter) defaultLabels() map[string]string {
	return map[string]string{
		"app.kubernetes.io/name": ad.Config.BaseName,
	}
}

func makePodSpec(baseName, image string, opts ...commonresources.PodSpecOption) corev1.PodSpec {
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
					"--config.file=/etc/prometheus/prometheus.yml",
					"--storage.tsdb.path=/prometheus/",
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
				VolumeMounts: []corev1.VolumeMount{{Name: "prometheus-config-volume", MountPath: "/etc/prometheus/"}, {Name: "prometheus-storage-volume", MountPath: "/prometheus/"}},
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
					FailureThreshold: 5, PeriodSeconds: 5, TimeoutSeconds: 3, SuccessThreshold: 1,
				},
				ReadinessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{Path: "/-/ready", Port: intstr.IntOrString{IntVal: ports.PrometheusPort}},
					},
					FailureThreshold: 3, PeriodSeconds: 5, TimeoutSeconds: 3, SuccessThreshold: 1,
				},
			},
		},
		ServiceAccountName:            baseName,
		TerminationGracePeriodSeconds: ptr.To(int64(300)),
		SecurityContext: &corev1.PodSecurityContext{
			RunAsUser:    ptr.To(prometheusUser),
			RunAsNonRoot: ptr.To(true),
			SeccompProfile: &corev1.SeccompProfile{
				Type: corev1.SeccompProfileTypeRuntimeDefault,
			},
		},
		Volumes: []corev1.Volume{
			{
				Name: "prometheus-config-volume",
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
				Name: "prometheus-storage-volume",
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
			corev1.ResourceCPU:    cpuLimit,
			corev1.ResourceMemory: memoryLimit,
		},
		Requests: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    cpuRequest,
			corev1.ResourceMemory: memoryRequest,
		},
	}
}

func (ad *ApplierDeleter) makeService(port int) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ad.Config.BaseName,
			Namespace: ad.Config.Namespace,
			Labels:    ad.defaultLabels(),
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Protocol:   corev1.ProtocolTCP,
					Port:       int32(port),
					TargetPort: intstr.FromInt32(int32(port)),
				},
			},
			Selector: ad.defaultLabels(),
			Type:     corev1.ServiceTypeClusterIP,
		},
	}
}
