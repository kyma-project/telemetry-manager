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

func New(namespace string) *kitk8s.Pod {
	return kitk8s.NewPod("telemetrygen", namespace).WithPodSpec(PodSpec(SignalTypeMetrics, ""))
}

func PodSpec(signalType SignalType, serviceNameAttrValue string) corev1.PodSpec {
	var gatewayPushURL string
	if signalType == SignalTypeTraces {
		gatewayPushURL = "telemetry-otlp-traces.kyma-system:4317"
	} else if signalType == SignalTypeMetrics {
		gatewayPushURL = "telemetry-otlp-metrics.kyma-system:4317"
	}

	serviceNameAttr := fmt.Sprintf("service.name=\"%s\"", serviceNameAttrValue)

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
					serviceNameAttr,
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
