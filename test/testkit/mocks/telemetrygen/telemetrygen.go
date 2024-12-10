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
	SignalTypeLogs    = "logs"
	DefaultName       = "telemetrygen"
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

// WithMemoryLimits sets the memory limits for the telemetry generator
func WithMemoryLimits(memory string) Option {
	return func(spec *corev1.PodSpec) {
		spec.Containers[0].Resources.Limits[corev1.ResourceMemory] = resource.MustParse(memory)
	}
}

// WithRate sets the rate of the telemetry generator (in events per second)
func WithRate(rate int) Option {
	return func(spec *corev1.PodSpec) {
		// find the rate argument and replace it
		for i, arg := range spec.Containers[0].Args {
			if arg == "--rate" {
				spec.Containers[0].Args[i+1] = fmt.Sprintf("%d", rate)
				return
			}
		}
	}
}

// WithWorkers sets the number of workers in the telemetry generator
func WithWorkers(workers int) Option {
	return func(spec *corev1.PodSpec) {
		spec.Containers[0].Args = append(spec.Containers[0].Args, "--workers")
		spec.Containers[0].Args = append(spec.Containers[0].Args, fmt.Sprintf("%v", workers))
	}
}

// WithSpanSize sets the size (in MB) of the spans generated by the telemetry generator
func WithSpanSize(spanSize int) Option {
	return func(spec *corev1.PodSpec) {
		spec.Containers[0].Args = append(spec.Containers[0].Args, "--size")
		spec.Containers[0].Args = append(spec.Containers[0].Args, fmt.Sprintf("%v", spanSize))
	}
}

// WithInterval Reporting interval
func WithInterval(duration string) Option {
	return func(spec *corev1.PodSpec) {
		spec.Containers[0].Args = append(spec.Containers[0].Args, "--interval")
		spec.Containers[0].Args = append(spec.Containers[0].Args, fmt.Sprintf("%v", duration))
	}
}
func NewPod(namespace string, signalType SignalType, opts ...Option) *kitk8s.Pod {
	return kitk8s.NewPod(DefaultName, namespace).WithPodSpec(PodSpec(signalType, opts...)).WithLabel("app.kubernetes.io/name", DefaultName)
}

func NewDeployment(namespace string, signalType SignalType, opts ...Option) *kitk8s.Deployment {
	return kitk8s.NewDeployment(DefaultName, namespace).WithPodSpec(PodSpec(signalType, opts...)).WithLabel("app.kubernetes.io/name", DefaultName)
}

func PodSpec(signalType SignalType, opts ...Option) corev1.PodSpec {
	var gatewayPushURL string

	switch signalType {
	case SignalTypeTraces:
		gatewayPushURL = "telemetry-otlp-traces.kyma-system:4317"
	case SignalTypeMetrics:
		gatewayPushURL = "telemetry-otlp-metrics.kyma-system:4317"
	case SignalTypeLogs:
		gatewayPushURL = "telemetry-otlp-logs.kyma-system:4317"
	}

	spec := corev1.PodSpec{

		Containers: []corev1.Container{
			{
				Name:  "telemetrygen",
				Image: "ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen:v0.115.0",
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
				ImagePullPolicy: corev1.PullIfNotPresent,
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
