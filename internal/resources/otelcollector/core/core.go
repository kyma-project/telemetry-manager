package core

import (
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"

	collectorconfig "github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
)

const (
	configMapKey           = "relay.conf"
	collectorUser          = 10001
	collectorContainerName = "collector"
)

func MakeDefaultLabels(baseName string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name": baseName,
	}
}

type PodSpecOption = func(pod *corev1.PodSpec)

func WithAffinity(affinity corev1.Affinity) PodSpecOption {
	return func(pod *corev1.PodSpec) {
		pod.Affinity = &affinity
	}
}

func WithCurrentPodIPEnvVar(envVarName string) PodSpecOption {
	return func(pod *corev1.PodSpec) {
		pod.Containers[0].Env = append(pod.Containers[0].Env, corev1.EnvVar{
			Name: envVarName,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath:  "status.podIP",
					APIVersion: "v1",
				},
			},
		})
	}
}

func WithCurrentNodeNameEnvVar(envVarName string) PodSpecOption {
	return func(pod *corev1.PodSpec) {
		pod.Containers[0].Env = append(pod.Containers[0].Env, corev1.EnvVar{
			Name: envVarName,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath:  "spec.nodeName",
					APIVersion: "v1",
				},
			},
		})
	}
}

func WithPriorityClass(priorityClassName string) PodSpecOption {
	return func(pod *corev1.PodSpec) {
		pod.PriorityClassName = priorityClassName
	}
}

func WithResources(resources corev1.ResourceRequirements) PodSpecOption {
	return func(pod *corev1.PodSpec) {
		for i := range pod.Containers {
			pod.Containers[i].Resources = resources
		}
	}
}

func MakePodSpec(baseName, image string, opts ...PodSpecOption) corev1.PodSpec {
	pod := corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:  collectorContainerName,
				Image: image,
				Args:  []string{"--config=/conf/" + configMapKey},
				EnvFrom: []corev1.EnvFromSource{
					{
						SecretRef: &corev1.SecretEnvSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: baseName,
							},
							Optional: pointer.Bool(true),
						},
					},
				},
				SecurityContext: &corev1.SecurityContext{
					Privileged:               pointer.Bool(false),
					RunAsUser:                pointer.Int64(collectorUser),
					RunAsNonRoot:             pointer.Bool(true),
					ReadOnlyRootFilesystem:   pointer.Bool(true),
					AllowPrivilegeEscalation: pointer.Bool(false),
					SeccompProfile: &corev1.SeccompProfile{
						Type: corev1.SeccompProfileTypeRuntimeDefault,
					},
					Capabilities: &corev1.Capabilities{
						Drop: []corev1.Capability{"ALL"},
					},
				},
				VolumeMounts: []corev1.VolumeMount{{Name: "config", MountPath: "/conf"}},
				LivenessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{Path: "/", Port: intstr.IntOrString{IntVal: 13133}},
					},
				},
				ReadinessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{Path: "/", Port: intstr.IntOrString{IntVal: 13133}},
					},
				},
			},
		},
		ServiceAccountName: baseName,
		SecurityContext: &corev1.PodSecurityContext{
			RunAsUser:    pointer.Int64(collectorUser),
			RunAsNonRoot: pointer.Bool(true),
			SeccompProfile: &corev1.SeccompProfile{
				Type: corev1.SeccompProfileTypeRuntimeDefault,
			},
		},
		Volumes: []corev1.Volume{
			{
				Name: "config",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: baseName,
						},
						Items: []corev1.KeyToPath{{Key: configMapKey, Path: configMapKey}},
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

func MakeCommonPodAnnotations(configHash string) map[string]string {
	annotations := map[string]string{
		"checksum/config":         configHash,
		"sidecar.istio.io/inject": "false",
	}

	return annotations
}

func MakeConfigMap(name types.NamespacedName, collectorConfig collectorconfig.Config) *corev1.ConfigMap {
	bytes, _ := yaml.Marshal(collectorConfig)
	confYAML := string(bytes)

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
			Labels:    MakeDefaultLabels(name.Name),
		},
		Data: map[string]string{
			configMapKey: confYAML,
		},
	}
}
