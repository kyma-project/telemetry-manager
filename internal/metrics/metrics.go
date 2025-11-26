package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	metricNamespace         = "telemetry"
	metricSubsystem         = "pipelines"
	selfMonitorMetricPrefix = "telemetry_self_monitor_prober_"
)

var (
	// OTTLTransformUsage tracks the number of pipelines using OTTL transform feature
	OTTLTransformUsage = promauto.With(Registry).NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: metricNamespace,
			Subsystem: metricSubsystem,
			Name:      "ottl_transform_usage",
			Help:      "Number of pipelines using OTTL transform feature",
		},
		[]string{"pipeline_type"},
	)

	// OTTLFilterUsage tracks the number of pipelines using OTTL filter feature
	OTTLFilterUsage = promauto.With(Registry).NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: metricNamespace,
			Subsystem: metricSubsystem,
			Name:      "ottl_filter_usage",
			Help:      "Number of pipelines using OTTL filter feature",
		},
		[]string{"pipeline_type"},
	)

	// SelfMonitorProberRequestsInFlight tracks the current number of in-flight requests initiated by the self-monitoring prober
	SelfMonitorProberRequestsInFlight = promauto.With(Registry).NewGauge(
		prometheus.GaugeOpts{
			Name: selfMonitorMetricPrefix + "in_flight_requests",
			Help: "The current number of in-flight requests initiated by the self-monitoring prober.",
		},
	)

	// SelfMonitorProberRequestsTotal tracks the total number of requests initiated by the self-monitoring prober
	SelfMonitorProberRequestsTotal = promauto.With(Registry).NewCounterVec(
		prometheus.CounterOpts{
			Name: selfMonitorMetricPrefix + "requests_total",
			Help: "Total number of requests initiated by the self-monitoring prober.",
		},
		[]string{"code"},
	)

	// SelfMonitorProberRequestDuration tracks the latency histogram for requests initiated by the self-monitoring prober
	SelfMonitorProberRequestDuration = promauto.With(Registry).NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    selfMonitorMetricPrefix + "duration_seconds",
			Help:    "A histogram of latencies for requests initiated by the self-monitoring prober.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{},
	)
)

// Registry is the prometheus registry for telemetry manager metrics
var Registry = ctrlmetrics.Registry

// RecordOTTLTransformUsage updates the transform usage metric for a given pipeline type
func RecordOTTLTransformUsage(pipelineType string, count int) {
	OTTLTransformUsage.WithLabelValues(pipelineType).Set(float64(count))
}

// RecordOTTLFilterUsage updates the filter usage metric for a given pipeline type
func RecordOTTLFilterUsage(pipelineType string, count int) {
	OTTLFilterUsage.WithLabelValues(pipelineType).Set(float64(count))
}
