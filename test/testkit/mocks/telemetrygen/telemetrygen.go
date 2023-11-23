package telemetrygen

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type SignalType string

const (
	SignalTypeTraces  = "traces"
	SignalTypeMetrics = "metrics"
)

func PodSpec(signalType SignalType) corev1.PodSpec {
	var gatewayPushURL string
	if signalType == SignalTypeTraces {
		gatewayPushURL = "telemetry-otlp-traces.kyma-system:4317"
	} else if signalType == SignalTypeMetrics {
		gatewayPushURL = "telemetry-otlp-metrics.kyma-system:4317"
	}

	return corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:  "telemetrygen",
				Image: "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen:v0.87.0",
				Args: []string{
					string(signalType),
					"--rate",
					"10",
					"--duration",
					"30m",
					"--otlp-endpoint",
					gatewayPushURL,
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
