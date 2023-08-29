package metricproducer

import (
	"strconv"

	"go.opentelemetry.io/collector/pdata/pmetric"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type Metric struct {
	Type   pmetric.MetricType
	Name   string
	Labels []string
}

var (
	MetricCPUTemperature = Metric{
		Type: pmetric.MetricTypeGauge,
		Name: "cpu_temperature_celsius",
	}
	MetricHardDiskErrorsTotal = Metric{
		Type:   pmetric.MetricTypeSum,
		Name:   "hd_errors_total",
		Labels: []string{"device"},
	}
	MetricCPUEnergyHistogram = Metric{
		Type:   pmetric.MetricTypeHistogram,
		Name:   "cpu_energy_watt",
		Labels: []string{"core"},
	}
	MetricHardwareHumidity = Metric{
		Type:   pmetric.MetricTypeSummary,
		Name:   "hw_humidity",
		Labels: []string{"sensor"},
	}

	metricsPort           = 8080
	metricsPortName       = "http-metrics"
	metricsEndpoint       = "/metrics"
	baseName              = "metric-producer"
	prometheusAnnotations = map[string]string{
		"prometheus.io/path":   metricsEndpoint,
		"prometheus.io/port":   strconv.Itoa(metricsPort),
		"prometheus.io/scrape": "true",
		"prometheus.io/scheme": "http",
	}
	selectorLabels = map[string]string{
		"app": "sample-metrics",
	}
)

// MetricProducer represents a workload that exposes dummy metrics in the Prometheus exposition format
type MetricProducer struct {
	namespace string
}

func (mp *MetricProducer) Name() string {
	return baseName
}

func (mp *MetricProducer) MetricsEndpoint() string {
	return metricsEndpoint
}

func (mp *MetricProducer) MetricsPort() int {
	return metricsPort
}

type Pod struct {
	namespace   string
	annotations map[string]string
}

type Service struct {
	namespace   string
	annotations map[string]string
}

func New(namespace string) *MetricProducer {
	return &MetricProducer{
		namespace: namespace,
	}
}

func (mp *MetricProducer) Pod() *Pod {
	return &Pod{
		namespace: mp.namespace,
	}
}

func (p *Pod) WithPrometheusAnnotations() *Pod {
	p.annotations = prometheusAnnotations
	return p
}

func (p *Pod) K8sObject() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        baseName,
			Namespace:   p.namespace,
			Labels:      selectorLabels,
			Annotations: p.annotations,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "sample-metrics",
					Image: "ckleineweber076/monitoring-custom-metrics:otlp-tracing",
					Ports: []corev1.ContainerPort{
						{
							Name:          metricsPortName,
							ContainerPort: int32(metricsPort),
							Protocol:      corev1.ProtocolTCP,
						},
					},
					Env: []corev1.EnvVar{
						{
							Name:  "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT",
							Value: "http://telemetry-otlp-traces.kyma-system:4318/v1/traces",
						},
					},
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("100Mi"),
						},
						Requests: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("32Mi"),
						},
					},
				},
			},
		},
	}
}

func (mp *MetricProducer) Service() *Service {
	return &Service{
		namespace: mp.namespace,
	}
}

func (s *Service) WithPrometheusAnnotations() *Service {
	s.annotations = prometheusAnnotations
	return s
}

func (s *Service) K8sObject() *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        baseName,
			Namespace:   s.namespace,
			Annotations: s.annotations,
			Labels:      selectorLabels,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       metricsPortName,
					Protocol:   corev1.ProtocolTCP,
					Port:       int32(metricsPort),
					TargetPort: intstr.FromString(metricsPortName),
				},
			},
			Selector: selectorLabels,
			Type:     corev1.ServiceTypeClusterIP,
		},
	}
}
