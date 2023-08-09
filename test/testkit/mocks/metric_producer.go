package mocks

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type MetricType string

const (
	MetricTypeGauge     MetricType = "Gauge"
	MetricTypeCounter   MetricType = "Counter"
	MetricTypeHistogram MetricType = "Histogram"
	MetricTypeSummary   MetricType = "Summary"
)

type CustomMetric struct {
	Type   MetricType
	Name   string
	Labels []string
}

var (
	CustomMetricCPUTemperature = CustomMetric{
		Type: MetricTypeGauge,
		Name: "cpu_temperature_celsius",
	}
	CustomMetricHardDiskErrorsTotal = CustomMetric{
		Type:   MetricTypeCounter,
		Name:   "hd_errors_total",
		Labels: []string{"device"},
	}
	CustomMetricCPUEnergyHistogram = CustomMetric{
		Type:   MetricTypeHistogram,
		Name:   "cpu_energy_watt",
		Labels: []string{"core"},
	}
	CustomMetricHardwareHumidity = CustomMetric{
		Type:   MetricTypeSummary,
		Name:   "hw_humidity",
		Labels: []string{"sensor"},
	}

	metricProducerName    = "metric-producer"
	prometheusAnnotations = map[string]string{
		"prometheus.io/path":   "/metrics",
		"prometheus.io/port":   "8080",
		"prometheus.io/scrape": "true",
		"prometheus.io/scheme": "http",
	}
	selectorLabels = map[string]string{
		"app": "sample-metrics",
	}
	metricsPort     int32 = 8080
	metricsPortName       = "http-metrics"
)

// MetricProducer represents a workload that exposes dummy metrics in the Prometheus exposition format
type MetricProducer struct {
	namespace string
}

type MetricProducerPod struct {
	namespace   string
	annotations map[string]string
}

type MetricProducerService struct {
	namespace   string
	annotations map[string]string
}

func NewMetricProducer(namespace string) *MetricProducer {
	return &MetricProducer{
		namespace: namespace,
	}
}

func (mp *MetricProducer) Pod() *MetricProducerPod {
	return &MetricProducerPod{
		namespace: mp.namespace,
	}
}

func (p *MetricProducerPod) WithPrometheusAnnotations() {
	p.annotations = prometheusAnnotations
}

func (p *MetricProducerPod) K8sObject() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        metricProducerName,
			Namespace:   p.namespace,
			Labels:      selectorLabels,
			Annotations: p.annotations,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  metricProducerName,
					Image: "ghcr.io/skhalash/examples/monitoring-custom-metrics:3d41736",
					Ports: []corev1.ContainerPort{
						{
							Name:          metricsPortName,
							ContainerPort: metricsPort,
							Protocol:      corev1.ProtocolTCP,
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

func (mp *MetricProducer) Service() *MetricProducerService {
	return &MetricProducerService{
		namespace: mp.namespace,
	}
}

func (s *MetricProducerService) WithPrometheusAnnotations() {
	s.annotations = prometheusAnnotations
}

func (s *MetricProducerService) K8sObject() *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        metricProducerName,
			Namespace:   s.namespace,
			Annotations: s.annotations,
			Labels:      selectorLabels,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       metricsPortName,
					Protocol:   corev1.ProtocolTCP,
					Port:       metricsPort,
					TargetPort: intstr.FromString(metricsPortName),
				},
			},
			Selector: selectorLabels,
			Type:     corev1.ServiceTypeClusterIP,
		},
	}
}
