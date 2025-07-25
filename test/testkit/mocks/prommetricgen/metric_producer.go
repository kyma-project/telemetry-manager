package prommetricgen

import (
	"maps"
	"strconv"

	"go.opentelemetry.io/collector/pdata/pmetric"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/kyma-project/telemetry-manager/test/testkit/apiserverproxy"
)

const (
	// A sample app instrumented with OpenTelemetry to generate metrics in the Prometheus exposition format
	// https://github.com/kyma-project/telemetry-manager/tree/main/docs/user/integration/sample-app
	metricProducerImage = "europe-docker.pkg.dev/kyma-project/prod/samples/telemetry-sample-app:latest"
)

type Metric struct {
	Type   pmetric.MetricType
	Name   string
	Labels []string
}

type ScrapingScheme string

const (
	SchemeHTTP  ScrapingScheme = "http"
	SchemeHTTPS ScrapingScheme = "https"
	SchemeNone  ScrapingScheme = "none"
)

var (
	MetricCPUTemperature = Metric{
		Type: pmetric.MetricTypeGauge,
		Name: "cpu.temperature.celsius",
	}

	MetricHardDiskErrorsTotal = Metric{
		Type:   pmetric.MetricTypeSum,
		Name:   "hd.errors_total",
		Labels: []string{"device"},
	}

	MetricCPUEnergyHistogram = Metric{
		Type:   pmetric.MetricTypeHistogram,
		Name:   "cpu.energy.watt",
		Labels: []string{"core"},
	}

	MetricPromhttpMetricHandlerRequestsTotal = Metric{
		Type:   pmetric.MetricTypeSum,
		Name:   "promhttp.metric.handler.requests.url_params_total",
		Labels: []string{"name", "value"},
	}
	MetricPromhttpMetricHandlerRequestsTotalLabelKey = "name"
	MetricPromhttpMetricHandlerRequestsTotalLabelVal = "value"

	// For each configured URL parameter, the MetricPromhttpMetricHandlerRequestsTotal metric
	// will include a label with the parameter name and a corresponding label with its value.
	ScrapingURLParamName = "format"
	ScrapingURLParamVal  = "prometheus"

	metricsPort     int32 = 8080
	metricsPortName       = "http-metrics"
	metricsEndpoint       = "/metrics"
	selectorLabels        = map[string]string{
		"app.kubernetes.io/name": "metric-producer",
	}
)

func CustomMetrics() []Metric {
	return []Metric{
		MetricCPUTemperature,
		MetricHardDiskErrorsTotal,
		MetricCPUEnergyHistogram,
		MetricPromhttpMetricHandlerRequestsTotal,
	}
}

func CustomMetricNames() []string {
	metrics := CustomMetrics()
	names := make([]string, len(metrics))

	for i, metric := range metrics {
		names[i] = metric.Name
	}

	return names
}

// MetricProducer represents a workload that exposes dummy metrics in the Prometheus exposition format
type MetricProducer struct {
	name      string
	namespace string
	labels    map[string]string
}

func (mp *MetricProducer) PodURL(proxyClient *apiserverproxy.Client) string {
	return proxyClient.ProxyURLForPod(mp.namespace, mp.name, mp.MetricsEndpoint(), mp.MetricsPort())
}

func (mp *MetricProducer) Name() string {
	return mp.name
}

func (mp *MetricProducer) MetricsEndpoint() string {
	return metricsEndpoint
}

func (mp *MetricProducer) MetricsPort() int32 {
	return metricsPort
}

type Pod struct {
	name        string
	namespace   string
	labels      map[string]string
	annotations map[string]string
}

type Service struct {
	name        string
	namespace   string
	annotations map[string]string
}

type Option = func(mp *MetricProducer)

func WithName(name string) Option {
	return func(mp *MetricProducer) {
		mp.name = name
	}
}

func New(namespace string, opts ...Option) *MetricProducer {
	mp := &MetricProducer{
		name:      "metric-producer",
		namespace: namespace,
		labels:    make(map[string]string),
	}
	for _, opt := range opts {
		opt(mp)
	}

	return mp
}

func (mp *MetricProducer) Pod() *Pod {
	return &Pod{
		name:        mp.name,
		namespace:   mp.namespace,
		labels:      make(map[string]string),
		annotations: make(map[string]string),
	}
}

func (p *Pod) WithPrometheusAnnotations(scheme ScrapingScheme) *Pod {
	maps.Copy(p.annotations, makePrometheusAnnotations(scheme))
	return p
}

func (p *Pod) WithSidecarInjection() *Pod {
	return p.WithLabel("sidecar.istio.io/inject", "true")
}

func (p *Pod) WithLabel(key, value string) *Pod {
	p.labels[key] = value
	return p
}

func makePrometheusAnnotations(scheme ScrapingScheme) map[string]string {
	annotations := map[string]string{
		"prometheus.io/scrape":                        "true",
		"prometheus.io/path":                          metricsEndpoint,
		"prometheus.io/port":                          strconv.Itoa(int(metricsPort)),
		"prometheus.io/param_" + ScrapingURLParamName: ScrapingURLParamVal,
	}
	if scheme != SchemeNone {
		annotations["prometheus.io/scheme"] = string(scheme)
	}

	return annotations
}

func (p *Pod) WithLabels(labels map[string]string) *Pod {
	maps.Copy(p.labels, labels)
	return p
}

func (p *Pod) K8sObject() *corev1.Pod {
	labels := p.labels
	maps.Copy(labels, selectorLabels)

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        p.name,
			Namespace:   p.namespace,
			Labels:      labels,
			Annotations: p.annotations,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "metric-producer",
					Image: metricProducerImage,
					Ports: []corev1.ContainerPort{
						{
							Name:          metricsPortName,
							ContainerPort: metricsPort,
							Protocol:      corev1.ProtocolTCP,
						},
					},
					Env: []corev1.EnvVar{
						{
							Name:  "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT",
							Value: "http://telemetry-otlp-traces.kyma-system:4317",
						},
						{
							Name:  "OTEL_SERVICE_NAME",
							Value: "metric-producer",
						},
						{
							Name:  "OTEL_METRICS_EXPORTER",
							Value: "prometheus",
						},
					},
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("100Mi"),
						},
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("50m"),
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
		name:      mp.name,
		namespace: mp.namespace,
	}
}

func (s *Service) WithPrometheusAnnotations(scheme ScrapingScheme) *Service {
	s.annotations = makePrometheusAnnotations(scheme)
	return s
}

func (s *Service) K8sObject() *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        s.name,
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
