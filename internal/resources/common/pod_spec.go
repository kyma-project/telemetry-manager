package common

import (
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
)

type PodSpecOption func(*corev1.PodSpec)

func WithAffinity(affinity corev1.Affinity) PodSpecOption {
	return func(pod *corev1.PodSpec) {
		pod.Affinity = &affinity
	}
}

func WithArgs(args []string) PodSpecOption {
	return func(pod *corev1.PodSpec) {
		pod.Containers[0].Args = args
	}
}

func WithContainerName(name string) PodSpecOption {
	return func(pod *corev1.PodSpec) {
		pod.Containers[0].Name = name
	}
}

func WithEnvVarFromField(envVarName, fieldPath string) PodSpecOption {
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

func WithEnvVarsFromSecret(secretName string) PodSpecOption {
	return func(pod *corev1.PodSpec) {
		pod.Containers[0].EnvFrom = append(pod.Containers[0].EnvFrom, corev1.EnvFromSource{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: secretName,
				},
				Optional: ptr.To(true),
			},
		})
	}
}

func WithGoMemLimitEnvVar(memory resource.Quantity) PodSpecOption {
	goMemLimit := memory.Value() / 100 * 80 //nolint:mnd // 80% of memory

	return func(pod *corev1.PodSpec) {
		pod.Containers[0].Env = append(pod.Containers[0].Env, corev1.EnvVar{
			Name:  config.EnvVarGoMemLimit,
			Value: strconv.FormatInt(goMemLimit, 10),
		})
	}
}

func WithPodSecurityContext(securityContext *corev1.PodSecurityContext) PodSpecOption {
	return func(pod *corev1.PodSpec) {
		pod.SecurityContext = securityContext
	}
}

func WithPriorityClass(priorityClassName string) PodSpecOption {
	return func(pod *corev1.PodSpec) {
		pod.PriorityClassName = priorityClassName
	}
}

func WithProbes(liveness, readiness *corev1.Probe) PodSpecOption {
	return func(pod *corev1.PodSpec) {
		if liveness != nil {
			pod.Containers[0].LivenessProbe = liveness
		}

		if readiness != nil {
			pod.Containers[0].ReadinessProbe = readiness
		}
	}
}

func WithResources(resources corev1.ResourceRequirements) PodSpecOption {
	return func(pod *corev1.PodSpec) {
		pod.Containers[0].Resources = resources
	}
}

func WithRunAsUser(userID int64) PodSpecOption {
	return func(pod *corev1.PodSpec) {
		pod.Containers[0].SecurityContext = &corev1.SecurityContext{
			RunAsUser: ptr.To(userID),
		}
	}
}

func WithSecurityContext(securityContext *corev1.SecurityContext) PodSpecOption {
	return func(pod *corev1.PodSpec) {
		pod.Containers[0].SecurityContext = securityContext
	}
}

func WithVolumeMounts(volumeMounts []corev1.VolumeMount) PodSpecOption {
	return func(pod *corev1.PodSpec) {
		pod.Containers[0].VolumeMounts = append(pod.Containers[0].VolumeMounts, volumeMounts...)
	}
}

func WithVolumes(volumes []corev1.Volume) PodSpecOption {
	return func(pod *corev1.PodSpec) {
		pod.Volumes = append(pod.Volumes, volumes...)
	}
}

func MakePodSpec(baseName, image string, opts ...PodSpecOption) corev1.PodSpec {
	pod := corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:  baseName,
				Image: image,
			},
		},
		ServiceAccountName: baseName,
		SecurityContext: &corev1.PodSecurityContext{
			RunAsNonRoot: ptr.To(true),
			SeccompProfile: &corev1.SeccompProfile{
				Type: corev1.SeccompProfileTypeRuntimeDefault,
			},
		},
	}

	for _, opt := range opts {
		opt(&pod)
	}

	return pod
}
