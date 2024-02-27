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
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/configchecksum"
	"github.com/kyma-project/telemetry-manager/internal/k8sutils"
)

const (
	prometheusUser         int64 = 10001
	collectorContainerName       = "self-monitor"
	replicas               int32 = 1
)

type podSpecOption = func(pod *corev1.PodSpec)

func ApplyResources(ctx context.Context, c client.Client, config *Config) error {

	name := types.NamespacedName{Namespace: config.Namespace, Name: config.BaseName}

	// Create RBAC resources in the following order: service account, cluster role, cluster role binding.
	if err := k8sutils.CreateOrUpdateServiceAccount(ctx, c, makeServiceAccount(name)); err != nil {
		return fmt.Errorf("failed to create self-monitor service account: %w", err)
	}

	if err := k8sutils.CreateOrUpdateRole(ctx, c, makeRole(name)); err != nil {
		return fmt.Errorf("failed to create self-monitor cluster role: %w", err)
	}

	if err := k8sutils.CreateOrUpdateClusterRoleBinding(ctx, c, makeClusterRoleBinding(name)); err != nil {
		return fmt.Errorf("failed to create self-monitor cluster role binding: %w", err)
	}

	if err := k8sutils.CreateOrUpdateNetworkPolicy(ctx, c, makeNetworkPolicy(name, defaultLabels(name.Name))); err != nil {
		return fmt.Errorf("failed to create self-monitor network policy: %w", err)
	}

	configMap := makeConfigMap(name, config.monitoringConfig)
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

func makeClusterRoleBinding(name types.NamespacedName) *rbacv1.ClusterRoleBinding {
	clusterRoleBinding := rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
			Labels:    defaultLabels(name.Name),
		},
		Subjects: []rbacv1.Subject{{Name: name.Name, Namespace: name.Namespace, Kind: rbacv1.ServiceAccountKind}},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     name.Name,
		},
	}
	return &clusterRoleBinding
}

func makeNetworkPolicy(name types.NamespacedName, labels map[string]string) *networkingv1.NetworkPolicy {
	allowedPorts := []int32{int32(9090)}

	telemetryPodSelector := map[string]string{
		"app.kubernetes.io/instance": "telemetry",
		"control-plane":              "telemetry-operator",
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
							PodSelector: &metav1.LabelSelector{
								MatchLabels: telemetryPodSelector,
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
	selectorLabels := defaultLabels(cfg.BaseName)
	podLabels := maps.Clone(selectorLabels)
	podLabels["sidecar.istio.io/inject"] = "false"

	annotations := map[string]string{"checksum/config": configChecksum}
	resources := makeResourceRequirements(cfg)
	affinity := makePodAffinity(selectorLabels)
	podSpec := makePodSpec(cfg.BaseName, cfg.Deployment.Image,
		withPriorityClass(cfg.Deployment.PriorityClassName),
		withResources(resources),
		withAffinity(affinity),
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

func withPriorityClass(priorityClassName string) podSpecOption {
	return func(pod *corev1.PodSpec) {
		pod.PriorityClassName = priorityClassName
	}
}

func makePodAffinity(labels map[string]string) corev1.Affinity {
	return corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
				{
					Weight: 100,
					PodAffinityTerm: corev1.PodAffinityTerm{
						TopologyKey: "kubernetes.io/hostname",
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: labels,
						},
					},
				},
				{
					Weight: 100,
					PodAffinityTerm: corev1.PodAffinityTerm{
						TopologyKey: "topology.kubernetes.io/zone",
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: labels,
						},
					},
				},
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
	pod := corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:  collectorContainerName,
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
						HTTPGet: &corev1.HTTPGetAction{Path: "/-/ready", Port: intstr.IntOrString{IntVal: 9090}},
					},
				},
				ReadinessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{Path: "/-/ready", Port: intstr.IntOrString{IntVal: 9090}},
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

func withResources(resources corev1.ResourceRequirements) podSpecOption {
	return func(pod *corev1.PodSpec) {
		for i := range pod.Containers {
			pod.Containers[i].Resources = resources
		}
	}
}

func withAffinity(affinity corev1.Affinity) podSpecOption {
	return func(pod *corev1.PodSpec) {
		pod.Affinity = &affinity
	}
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
