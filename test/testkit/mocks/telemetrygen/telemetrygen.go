package telemetrygen

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pmetric"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
)

type SignalType string

type Metric struct {
	Type   pmetric.MetricType
	Name   string
	Labels []string
}

var (
	MetricGen = Metric{
		Type: pmetric.MetricTypeGauge,
		Name: "gen",
	}

	MetricNames = []string{
		MetricGen.Name,
	}
)

const (
	SignalTypeTraces  = "traces"
	SignalTypeMetrics = "metrics"
)

type Option func(*corev1.PodSpec)

func WithServiceName(serviceName string) Option {
	return WithResourceAttribute("service.name", serviceName)
}

func WithResourceAttribute(key, value string) Option {
	return func(spec *corev1.PodSpec) {
		spec.Containers[0].Args = append(spec.Containers[0].Args, "--otlp-attributes")
		spec.Containers[0].Args = append(spec.Containers[0].Args, fmt.Sprintf("%s=\"%s\"", key, value))
	}
}

func WithTelemetryAttribute(key, value string) Option {
	return func(spec *corev1.PodSpec) {
		spec.Containers[0].Args = append(spec.Containers[0].Args, "--telemetry-attributes")
		spec.Containers[0].Args = append(spec.Containers[0].Args, fmt.Sprintf("%s=\"%s\"", key, value))
	}
}

func New(namespace string, signalType SignalType, opts ...Option) *kitk8s.Pod {
	return kitk8s.NewPod("telemetrygen", namespace).WithPodSpec(PodSpec(signalType, opts...))
}

func PodSpec(signalType SignalType, opts ...Option) corev1.PodSpec {
	var gatewayPushURL string
	if signalType == SignalTypeTraces {
		gatewayPushURL = "telemetry-otlp-traces.kyma-system:4317"
	} else if signalType == SignalTypeMetrics {
		gatewayPushURL = "telemetry-otlp-metrics.kyma-system:4317"
	}

	spec := corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:  "telemetrygen",
				Image: "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen:v0.97.0",
				Args: []string{
					string(signalType),
					"--rate",
					"10",
					"--duration",
					"30m",
					"--otlp-endpoint",
					gatewayPushURL,
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

	for _, opt := range opts {
		opt(&spec)
	}

	return spec
}
