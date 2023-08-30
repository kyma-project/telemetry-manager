package agent

import (
	"fmt"
	"maps"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector/core"
)

const istioCertVolumeName = "istio-certs"

type Config struct {
	BaseName  string
	Namespace string

	DaemonSet DaemonSetConfig
}

type DaemonSetConfig struct {
	Image             string
	PriorityClassName string
	CPULimit          resource.Quantity
	CPURequest        resource.Quantity
	MemoryLimit       resource.Quantity
	MemoryRequest     resource.Quantity
}

func MakeClusterRole(name types.NamespacedName) *rbacv1.ClusterRole {
	clusterRole := rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"nodes", "nodes/metrics", "nodes/stats", "services", "endpoints", "pods"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				NonResourceURLs: []string{"/metrics", "/metrics/cadvisor"},
				Verbs:           []string{"get"},
			},
		},
	}
	return &clusterRole
}

func MakeDaemonSet(config Config, configHash, envVarPodIP, envVarNodeName, istioCertPath string) *appsv1.DaemonSet {
	labels := core.MakeDefaultLabels(config.BaseName)
	labels["sidecar.istio.io/inject"] = "true"

	annotations := core.MakeCommonPodAnnotations(configHash)
	maps.Copy(annotations, makeIstioTLSPodAnnotations(istioCertPath))

	resources := makeResourceRequirements(config)
	podSpec := core.MakePodSpec(config.BaseName, config.DaemonSet.Image,
		core.WithPriorityClass(config.DaemonSet.PriorityClassName),
		core.WithResources(resources),
		core.WithEnvVarFromSource(envVarPodIP, core.FieldPathPodIP),
		core.WithEnvVarFromSource(envVarNodeName, core.FieldPathNodeName),
		core.WithVolume(corev1.Volume{Name: istioCertVolumeName, VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		}}),
		core.WithVolumeMount(corev1.VolumeMount{
			Name:      istioCertVolumeName,
			MountPath: istioCertPath,
			ReadOnly:  true,
		}),
	)

	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.BaseName,
			Namespace: config.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: annotations,
				},
				Spec: podSpec,
			},
		},
	}
}

func makeResourceRequirements(config Config) corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Limits: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    config.DaemonSet.CPULimit,
			corev1.ResourceMemory: config.DaemonSet.MemoryLimit,
		},
		Requests: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    config.DaemonSet.CPURequest,
			corev1.ResourceMemory: config.DaemonSet.MemoryRequest,
		},
	}
}

func makeIstioTLSPodAnnotations(istioCertPath string) map[string]string {
	return map[string]string{
		"proxy.istio.io/config": fmt.Sprintf(`# configure an env variable OUTPUT_CERTS to write certificates to the given folder
proxyMetadata:
  OUTPUT_CERTS: %s
`, istioCertPath),
		"sidecar.istio.io/userVolumeMount":                 fmt.Sprintf(`[{"name": "%s", "mountPath": "%s"}]`, istioCertVolumeName, istioCertPath),
		"traffic.sidecar.istio.io/includeInboundPorts":     "",
		"traffic.sidecar.istio.io/includeOutboundIPRanges": "",
	}
}
