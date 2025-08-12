package common

import (
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
)

var (
	hardenedSecurityContext = corev1.SecurityContext{
		Privileged:               ptr.To(false),
		RunAsNonRoot:             ptr.To(true),
		ReadOnlyRootFilesystem:   ptr.To(true),
		AllowPrivilegeEscalation: ptr.To(false),
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		},
	}

	hardenedPodSecurityContext = corev1.PodSecurityContext{
		RunAsNonRoot: ptr.To(true),
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
	}
)

var (
	// CriticalDaemonSetTolerations is to be used for critical DaemonSets (e.g. log or metric collectors) that must run on all nodes,
	// even on nodes with NoSchedule or NoExecute taints.
	CriticalDaemonSetTolerations = []corev1.Toleration{
		{
			Effect:   corev1.TaintEffectNoExecute,
			Operator: corev1.TolerationOpExists,
		},
		{
			Effect:   corev1.TaintEffectNoSchedule,
			Operator: corev1.TolerationOpExists,
		},
	}
)

type PodSpecOption func(*corev1.PodSpec)

type ContainerOption func(*corev1.Container)

func WithAffinity(affinity corev1.Affinity) PodSpecOption {
	return func(pod *corev1.PodSpec) {
		pod.Affinity = &affinity
	}
}

func WithContainer(name, image string, opts ...ContainerOption) PodSpecOption {
	return func(pod *corev1.PodSpec) {
		container := corev1.Container{
			Name:            name,
			Image:           image,
			SecurityContext: hardenedSecurityContext.DeepCopy(),
		}

		for _, opt := range opts {
			opt(&container)
		}

		pod.Containers = append(pod.Containers, container)
	}
}

func WithVolumes(volumes []corev1.Volume) PodSpecOption {
	return func(pod *corev1.PodSpec) {
		pod.Volumes = append(pod.Volumes, volumes...)
	}
}

func WithPodRunAsUser(userID int64) PodSpecOption {
	return func(pod *corev1.PodSpec) {
		pod.SecurityContext.RunAsUser = ptr.To(userID)
	}
}

func WithPriorityClass(priorityClassName string) PodSpecOption {
	return func(pod *corev1.PodSpec) {
		pod.PriorityClassName = priorityClassName
	}
}

func WithTerminationGracePeriodSeconds(seconds int64) PodSpecOption {
	return func(pod *corev1.PodSpec) {
		pod.TerminationGracePeriodSeconds = &seconds
	}
}

func WithTolerations(tolerations []corev1.Toleration) PodSpecOption {
	return func(pod *corev1.PodSpec) {
		pod.Tolerations = append(pod.Tolerations, tolerations...)
	}
}

func WithArgs(args []string) ContainerOption {
	return func(c *corev1.Container) {
		c.Args = args
	}
}

func WithCapabilities(capabilities ...corev1.Capability) ContainerOption {
	return func(c *corev1.Container) {
		c.SecurityContext.Capabilities.Add = append(c.SecurityContext.Capabilities.Add, capabilities...)
	}
}

func WithPort(name string, port int32) ContainerOption {
	return func(c *corev1.Container) {
		c.Ports = append(c.Ports, corev1.ContainerPort{
			Name:          name,
			ContainerPort: port,
		})
	}
}

func WithEnvVarFromField(envVarName, fieldPath string) ContainerOption {
	return func(c *corev1.Container) {
		c.Env = append(c.Env, corev1.EnvVar{
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

func WithEnvVarsFromSecret(secretName string) ContainerOption {
	return func(c *corev1.Container) {
		c.EnvFrom = append(c.EnvFrom, corev1.EnvFromSource{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: secretName,
				},
				Optional: ptr.To(true),
			},
		})
	}
}

func WithGoMemLimitEnvVar(memory resource.Quantity) ContainerOption {
	goMemLimit := memory.Value() / 100 * 80 //nolint:mnd // 80% of the memory limit

	return func(c *corev1.Container) {
		c.Env = append(c.Env, corev1.EnvVar{
			Name:  common.EnvVarGoMemLimit,
			Value: strconv.FormatInt(goMemLimit, 10),
		})
	}
}

func WithResources(resources corev1.ResourceRequirements) ContainerOption {
	return func(c *corev1.Container) {
		c.Resources = resources
	}
}

func WithVolumeMounts(volumeMounts []corev1.VolumeMount) ContainerOption {
	return func(c *corev1.Container) {
		c.VolumeMounts = append(c.VolumeMounts, volumeMounts...)
	}
}

func WithProbes(liveness, readiness *corev1.Probe) ContainerOption {
	return func(c *corev1.Container) {
		if liveness != nil {
			c.LivenessProbe = liveness
		}

		if readiness != nil {
			c.ReadinessProbe = readiness
		}
	}
}

func WithRunAsRoot() ContainerOption {
	return func(c *corev1.Container) {
		c.SecurityContext.RunAsNonRoot = ptr.To(false)
	}
}

func WithRunAsUser(userID int64) ContainerOption {
	return func(c *corev1.Container) {
		c.SecurityContext.RunAsUser = ptr.To(userID)
	}
}

func MakePodSpec(baseName string, opts ...PodSpecOption) corev1.PodSpec {
	pod := corev1.PodSpec{
		ServiceAccountName: baseName,
		SecurityContext:    hardenedPodSecurityContext.DeepCopy(),
	}

	for _, opt := range opts {
		opt(&pod)
	}

	return pod
}
