package k8s

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func SleeperPodSpec() corev1.PodSpec {
	return corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:  "sleeper",
				Image: "busybox",
				Command: []string{
					"sh",
					"-c",
					"while true; do sleep 3600; done",
				},
			},
		},
	}
}

func TraceGenPodSpec() corev1.PodSpec {
	return corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:  "tracegen",
				Image: "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen:v0.87.0",
				Args: []string{
					"traces",
					"--rate",
					"10",
					"--duration",
					"30m",
					"--otlp-endpoint",
					"telemetry-otlp-traces.kyma-system:4317",
					"--otlp-attributes",
					"service.name=\"\"",
					"--otlp-insecure",
				},
				ImagePullPolicy: corev1.PullAlways,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("64Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
			},
		},
	}
}
