package core

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
)

const (
	configMapKey            = "relay.conf"
	configHashAnnotationKey = "checksum/config"
	collectorUser           = 10001
	collectorContainerName  = "collector"
)

func MakeDefaultLabels(baseName string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name": baseName,
	}
}

func MakePodSpec(baseName, image, priorityClassName string, resources corev1.ResourceRequirements) corev1.PodSpec {
	labels := MakeDefaultLabels(baseName)
	affinity := makePodAffinity(labels)

	return corev1.PodSpec{
		Affinity: &affinity,
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
				Env: []corev1.EnvVar{
					{
						Name: "MY_POD_IP",
						ValueFrom: &corev1.EnvVarSource{
							FieldRef: &corev1.ObjectFieldSelector{
								FieldPath:  "status.podIP",
								APIVersion: "v1",
							},
						},
					},
				},
				Resources: resources,
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
		PriorityClassName:  priorityClassName,
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
