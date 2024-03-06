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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/configchecksum"
	"github.com/kyma-project/telemetry-manager/internal/k8sutils"
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/ports"
)

type podSpecOption = func(pod *corev1.PodSpec)

func RemoveResources(ctx context.Context, c client.Client, config *Config) error {
	objectMeta := metav1.ObjectMeta{
		Name:      config.BaseName,
		Namespace: config.Namespace,
	}
	// Delete Deployment
	deployment := &appsv1.Deployment{ObjectMeta: objectMeta}
	if err := c.Delete(ctx, deployment); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	}
	// Delete Configmap
	configMap := &corev1.ConfigMap{ObjectMeta: objectMeta}
	if err := c.Delete(ctx, configMap); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	}
	// Delete Network policy
	networkPolicy := &networkingv1.NetworkPolicy{ObjectMeta: objectMeta}
	if err := c.Delete(ctx, networkPolicy); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	}

	// Delete RoleBinding
	roleBinding := &rbacv1.RoleBinding{ObjectMeta: objectMeta}
	if err := c.Delete(ctx, roleBinding); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	}
	// Delete Role
	role := &rbacv1.Role{ObjectMeta: objectMeta}
	if err := c.Delete(ctx, role); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	}

	// Delete service account
	serviceAccount := &corev1.ServiceAccount{ObjectMeta: objectMeta}
	if err := c.Delete(ctx, serviceAccount); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	}

	return nil
}

func ApplyResources(ctx context.Context, c client.Client, config *Config) error {
	name := types.NamespacedName{Namespace: config.Namespace, Name: config.BaseName}

	// Create RBAC resources in the following order: service account, cluster role, cluster role binding.
	if err := k8sutils.CreateOrUpdateServiceAccount(ctx, c, makeServiceAccount(name)); err != nil {
		return fmt.Errorf("failed to create self-monitor service account: %w", err)
	}

	if err := k8sutils.CreateOrUpdateRole(ctx, c, makeRole(name)); err != nil {
		return fmt.Errorf("failed to create self-monitor role: %w", err)
	}

	if err := k8sutils.CreateOrUpdateRoleBinding(ctx, c, makeRoleBinding(name)); err != nil {
		return fmt.Errorf("failed to create self-monitor role binding: %w", err)
	}

	if err := k8sutils.CreateOrUpdateNetworkPolicy(ctx, c, makeNetworkPolicyIngressPorts(name, defaultLabels(name.Name))); err != nil {
		return fmt.Errorf("failed to create self-monitor network policy: %w", err)
	}

	configMap := makeConfigMap(name, config.SelfMonitorConfig)
	if err := k8sutils.CreateOrUpdateConfigMap(ctx, c, configMap); err != nil {
		return fmt.Errorf("failed to create self-monitor configmap: %w", err)
	}

	checksum := configchecksum.Calculate([]corev1.ConfigMap{*configMap}, nil)
	if err := k8sutils.CreateOrUpdateDeployment(ctx, c, makeSelfMonitorDeployment(config, checksum)); err != nil {
		return fmt.Errorf("failed to create sel-monitor deployment: %w", err)
	}

	return nil
}

func makeServiceAccount(name types.NamespacedName) *corev1.ServiceAccount {
	serviceAccount := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
			Labels:    defaultLabels(name.Name),
		},
	}
	return &serviceAccount
}

func makeRoleBinding(name types.NamespacedName) *rbacv1.RoleBinding {
	roleBinding := rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
			Labels:    defaultLabels(name.Name),
		},
		Subjects: []rbacv1.Subject{{Name: name.Name, Namespace: name.Namespace, Kind: rbacv1.ServiceAccountKind}},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     name.Name,
		},
	}
	return &roleBinding
}

func makeNetworkPolicyIngressPorts(name types.NamespacedName, labels map[string]string) *networkingv1.NetworkPolicy {
	allowedPorts := []int32{int32(ports.PrometheusPort)}

	telemetryPodSelector := map[string]string{
		"self-monitor/access": "true",
	}
	namespaceSelector := map[string]string{
		"kubernetes.io/metadata.name": name.Namespace,
	}
	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
			Labels:    labels,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: labels,
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress,
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: namespaceSelector,
							},
							PodSelector: &metav1.LabelSelector{
								MatchLabels: telemetryPodSelector,
							},
						},
					},
					Ports: makeNetworkPolicyPorts(allowedPorts),
				},
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				{
					To: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: namespaceSelector,
							},
						},
					},
				},
			},
		},
	}
}

func makeNetworkPolicyPorts(ports []int32) []networkingv1.NetworkPolicyPort {
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

func makeConfigMap(name types.NamespacedName, selfmonitorConfig string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
			Labels:    defaultLabels(name.Name),
		},
		Data: map[string]string{
			"prometheus.yml": selfmonitorConfig,
		},
	}
}
func makeSelfMonitorDeployment(cfg *Config, configChecksum string) *appsv1.Deployment {
	var replicas int32 = 1
	selectorLabels := defaultLabels(cfg.BaseName)
	podLabels := maps.Clone(selectorLabels)
	podLabels["sidecar.istio.io/inject"] = "false"

	annotations := map[string]string{"checksum/config": configChecksum}
	resources := makeResourceRequirements(cfg)
	podSpec := makePodSpec(cfg.BaseName, cfg.Deployment.Image,
		commonresources.WithPriorityClass(cfg.Deployment.PriorityClassName),
		commonresources.WithResources(resources),
	)

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cfg.BaseName,
			Namespace: cfg.Namespace,
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

func makeRole(name types.NamespacedName) *rbacv1.Role {
	role := rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
			Labels:    defaultLabels(name.Name),
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

func defaultLabels(baseName string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name": baseName,
	}
}

func makePodSpec(baseName, image string, opts ...podSpecOption) corev1.PodSpec {
	var defaultMode int32 = 420
	var storageVolumeSize = resource.MustParse("500Mi")
	var prometheusUser int64 = 10001
	var containerName = "self-monitor"
	pod := corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:  containerName,
				Image: image,
				Args:  []string{"--storage.tsdb.retention.time=6h", "--config.file=/etc/prometheus/prometheus.yml", "--storage.tsdb.path=/prometheus/", "--web.enable-lifecycle"},
				EnvFrom: []corev1.EnvFromSource{
					{
						SecretRef: &corev1.SecretEnvSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: baseName,
							},
							Optional: ptr.To(true),
						},
					},
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
				LivenessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{Path: "/-/ready", Port: intstr.IntOrString{IntVal: ports.PrometheusPort}},
					},
				},
				ReadinessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{Path: "/-/ready", Port: intstr.IntOrString{IntVal: ports.PrometheusPort}},
					},
				},
			},
		},
		ServiceAccountName: baseName,
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

func makeResourceRequirements(cfg *Config) corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Limits: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    cfg.Deployment.CPULimit,
			corev1.ResourceMemory: cfg.Deployment.MemoryLimit,
		},
		Requests: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    cfg.Deployment.CPURequest,
			corev1.ResourceMemory: cfg.Deployment.MemoryRequest,
		},
	}
}
