package otelcollector

import (
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
)

const (
	configMapKey           = "relay.conf"
	collectorUser          = 10001
	collectorContainerName = "collector"
)

type podSpecOption = func(pod *corev1.PodSpec)

func withAffinity(affinity corev1.Affinity) podSpecOption {
	return func(pod *corev1.PodSpec) {
		pod.Affinity = &affinity
	}
}

const (
	fieldPathPodIP    = "status.podIP"
	fieldPathNodeName = "spec.nodeName"
)

func withEnvVarFromSource(envVarName, fieldPath string) podSpecOption {
	return func(pod *corev1.PodSpec) {
		pod.Containers[0].Env = append(pod.Containers[0].Env, corev1.EnvVar{
			Name: envVarName,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath:  fieldPath,
					APIVersion: "v1",
				},
			},
		})
	}
}

func withPriorityClass(priorityClassName string) podSpecOption {
	return func(pod *corev1.PodSpec) {
		pod.PriorityClassName = priorityClassName
	}
}

func withResources(resources corev1.ResourceRequirements) podSpecOption {
	return func(pod *corev1.PodSpec) {
		for i := range pod.Containers {
			pod.Containers[i].Resources = resources
		}
	}
}

func withVolumeMount(volumeMount corev1.VolumeMount) podSpecOption {
	return func(pod *corev1.PodSpec) {
		for i := range pod.Containers {
			pod.Containers[i].VolumeMounts = append(pod.Containers[i].VolumeMounts, volumeMount)
		}
	}
}

func withVolume(volume corev1.Volume) podSpecOption {
	return func(pod *corev1.PodSpec) {
		pod.Volumes = append(pod.Volumes, volume)
	}
}

func makePodSpec(baseName, image string, opts ...podSpecOption) corev1.PodSpec {
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
						HTTPGet: &corev1.HTTPGetAction{Path: "/", Port: intstr.IntOrString{IntVal: ports.HealthCheck}},
					},
				},
				ReadinessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{Path: "/", Port: intstr.IntOrString{IntVal: ports.HealthCheck}},
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

func makeCommonPodAnnotations(configHash string) map[string]string {
	annotations := map[string]string{
		"checksum/config": configHash,
	}

	return annotations
}
