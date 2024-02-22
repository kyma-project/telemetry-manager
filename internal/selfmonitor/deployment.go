package selfmonitor

import (
	"context"
	"fmt"
	"github.com/kyma-project/telemetry-manager/internal/configchecksum"
	"github.com/kyma-project/telemetry-manager/internal/k8sutils"
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"maps"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	prometheusUser         int64 = 10001
	collectorContainerName       = "prometheus"
)

type podSpecOption = func(pod *corev1.PodSpec)

type SelfMonitor struct {
	client            client.Client
	selfMonitorProber DeploymentProber
}

//go:generate mockery --name DeploymentProber --filename deployment_prober.go
type DeploymentProber interface {
	IsReady(ctx context.Context, name types.NamespacedName) (bool, error)
}

func NewSelfMonitor(client client.Client, selfMonitorProber DeploymentProber) *SelfMonitor {
	return &SelfMonitor{
		client:            client,
		selfMonitorProber: selfMonitorProber,
	}
}

func ApplyResources(ctx context.Context, c client.Client, config *PrometheusDeploymentConfig) error {

	name := types.NamespacedName{Namespace: config.Namespace, Name: config.BaseName}

	// Create RBAC resources in the following order: service account, cluster role, cluster role binding.
	if err := k8sutils.CreateOrUpdateServiceAccount(ctx, c, commonresources.MakeServiceAccount(name)); err != nil {
		return fmt.Errorf("failed to create service account: %w", err)
	}

	if err := k8sutils.CreateOrUpdateClusterRole(ctx, c, makeClusterRole(name)); err != nil {
		return fmt.Errorf("failed to create cluster role: %w", err)
	}

	if err := k8sutils.CreateOrUpdateClusterRoleBinding(ctx, c, commonresources.MakeClusterRoleBinding(name)); err != nil {
		return fmt.Errorf("failed to create cluster role binding: %w", err)
	}

	if err := k8sutils.CreateOrUpdateNetworkPolicy(ctx, c, commonresources.MakeNetworkPolicy(name, config.allowedPorts, defaultLabels(name.Name))); err != nil {
		return fmt.Errorf("failed to create deny pprof network policy: %w", err)
	}

	configMap := makeConfigMap(name, config.prometheusConfig)
	if err := k8sutils.CreateOrUpdateConfigMap(ctx, c, configMap); err != nil {
		return fmt.Errorf("failed to create configmap: %w", err)
	}

	checksum := configchecksum.CalculateWithConfigMaps([]corev1.ConfigMap{*configMap})
	if err := k8sutils.CreateOrUpdateDeployment(ctx, c, makeSelfMonitorDeployment(config, checksum)); err != nil {
		return fmt.Errorf("failed to create selmonitor deployment: %w", err)
	}

	return nil
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
func makeSelfMonitorDeployment(cfg *PrometheusDeploymentConfig, configChecksum string) *appsv1.Deployment {
	selectorLabels := defaultLabels(cfg.BaseName)
	podLabels := maps.Clone(selectorLabels)

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
			Replicas: ptr.To(cfg.Replicas),
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

func makeClusterRole(name types.NamespacedName) *rbacv1.ClusterRole {
	clusterRole := rbacv1.ClusterRole{
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
	return &clusterRole
}

// move to commonresources
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

func makeResourceRequirements(cfg *PrometheusDeploymentConfig) corev1.ResourceRequirements {
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
