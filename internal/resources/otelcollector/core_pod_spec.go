package otelcollector

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
)

const (
	collectorConfigFileName       = "relay.conf"
	collectorUser           int64 = 10001
	collectorContainerName        = "collector"
)

const (
	fieldPathPodIP    = "status.podIP"
	fieldPathNodeName = "spec.nodeName"
)

func makePodSpec(
	baseName,
	image string,
	podOpts []commonresources.PodSpecOption,
	containerOpts []commonresources.ContainerOption,
) corev1.PodSpec {
	volumes := []corev1.Volume{
		{
			Name: "config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: baseName,
					},
					Items: []corev1.KeyToPath{{Key: collectorConfigFileName, Path: collectorConfigFileName}},
				},
			},
		},
	}

	volumeMounts := []corev1.VolumeMount{{Name: "config", MountPath: "/conf"}}

	healthProbe := &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{Path: "/", Port: intstr.IntOrString{IntVal: ports.HealthCheck}},
		},
	}

	defaultContainerOpts := []commonresources.ContainerOption{
		commonresources.WithArgs([]string{"--config=/conf/" + collectorConfigFileName}),
		commonresources.WithEnvVarsFromSecret(baseName),
		commonresources.WithProbes(healthProbe, healthProbe),
		commonresources.WithRunAsUser(collectorUser),
		commonresources.WithVolumeMounts(volumeMounts),
	}
	containerOpts = append(defaultContainerOpts, containerOpts...)

	defaultPodOpts := []commonresources.PodSpecOption{
		commonresources.WithContainer(collectorContainerName, image, containerOpts...),
		commonresources.WithPodRunAsUser(collectorUser),
		commonresources.WithVolumes(volumes),
	}
	podOpts = append(defaultPodOpts, podOpts...)

	return commonresources.MakePodSpec(baseName, podOpts...)
}
