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

func makePodSpec(baseName, image string, opts ...commonresources.PodSpecOption) corev1.PodSpec {
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

	// commonOpts are shared options for all collectors
	commonOpts := []commonresources.PodSpecOption{
		commonresources.WithContainerName(collectorContainerName),
		commonresources.WithArgs([]string{"--config=/conf/" + collectorConfigFileName}),
		commonresources.WithEnvVarsFromSecret(baseName),
		commonresources.WithProbes(healthProbe, healthProbe),
		commonresources.WithRunAsUser(collectorUser),
		commonresources.WithVolumes(volumes),
		commonresources.WithVolumeMounts(volumeMounts),
	}

	opts = append(commonOpts, opts...)

	return commonresources.MakePodSpec(baseName, image, opts...)
}
