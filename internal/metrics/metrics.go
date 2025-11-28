package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/kyma-project/telemetry-manager/internal/build"
)

const (
	defaultNamespace           = "telemetry"
	subsystemPipelines         = "pipelines"
	subsystemSelfMonitorProber = "self_monitor_prober_"
)

const (
	// FeatureOTTL represents the OTTL (OpenTelemetry Transformation Language) feature
	FeatureOTTL = "ottl"
)

// registry is the prometheus registry for telemetry manager metrics
var registry = metrics.Registry

var (
	// BuildInfo provides build information of the Telemetry Manager
	BuildInfo = promauto.With(registry).NewGauge(
		prometheus.GaugeOpts{
			Namespace:   defaultNamespace,
			Subsystem:   "",
			Name:        "build_info",
			Help:        "Build information of the Telemetry Manager",
			ConstLabels: build.InfoMap(),
		},
	)

	// FeatureFlagsInfo tracks enabled feature flags in the Telemetry Manager
	FeatureFlagsInfo = promauto.With(registry).NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: defaultNamespace,
			Name:      "feature_flags_info",
			Help:      "Enabled feature flags in the Telemetry Manager",
		},
		[]string{"flag"},
	)

	// MetricPipelineFeatureUsage tracks the usage of features in MetricPipelines
	MetricPipelineFeatureUsage = promauto.With(registry).NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: defaultNamespace,
			Subsystem: subsystemPipelines,
			Name:      "metric_pipeline_feature_usage",
			Help:      "Number of MetricPipelines using specific features",
		},
		[]string{"feature", "pipeline_name"},
	)

	// LogPipelineFeatureUsage tracks the usage of features in LogPipelines
	LogPipelineFeatureUsage = promauto.With(registry).NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: defaultNamespace,
			Subsystem: subsystemPipelines,
			Name:      "log_pipeline_feature_usage",
			Help:      "Number of LogPipelines using specific features",
		},
		[]string{"feature", "pipeline_name"},
	)

	// TracePipelineFeatureUsage tracks the usage of features in TracePipelines
	TracePipelineFeatureUsage = promauto.With(registry).NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: defaultNamespace,
			Subsystem: subsystemPipelines,
			Name:      "trace_pipeline_feature_usage",
			Help:      "Number of TracePipelines using specific features",
		},
		[]string{"feature", "pipeline_name"},
	)

	// SelfMonitorProberRequestsInFlight tracks the current number of in-flight requests initiated by the self-monitoring prober
	SelfMonitorProberRequestsInFlight = promauto.With(registry).NewGauge(
		prometheus.GaugeOpts{
			Namespace: defaultNamespace,
			Subsystem: subsystemSelfMonitorProber,
			Name:      "in_flight_requests",
			Help:      "The current number of in-flight requests initiated by the self-monitoring prober.",
		},
	)

	// SelfMonitorProberRequestsTotal tracks the total number of requests initiated by the self-monitoring prober
	SelfMonitorProberRequestsTotal = promauto.With(registry).NewCounterVec(
		prometheus.CounterOpts{
			Namespace: defaultNamespace,
			Subsystem: subsystemSelfMonitorProber,
			Name:      "requests_total",
			Help:      "Total number of requests initiated by the self-monitoring prober.",
		},
		[]string{"code"},
	)

	// SelfMonitorProberRequestDuration tracks the latency histogram for requests initiated by the self-monitoring prober
	SelfMonitorProberRequestDuration = promauto.With(registry).NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: defaultNamespace,
			Subsystem: subsystemSelfMonitorProber,
			Name:      "duration_seconds",
			Help:      "A histogram of latencies for requests initiated by the self-monitoring prober.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{},
	)
)

// RecordMetricPipelineFeatureUsage updates the feature usage metric for MetricPipelines
func RecordMetricPipelineFeatureUsage(feature, pipelineName string, using bool) {
	recordFeatureUsage(MetricPipelineFeatureUsage, feature, pipelineName, using)
}

// RecordLogPipelineFeatureUsage updates the feature usage metric for LogPipelines
func RecordLogPipelineFeatureUsage(feature, pipelineName string, using bool) {
	recordFeatureUsage(LogPipelineFeatureUsage, feature, pipelineName, using)
}

// RecordTracePipelineFeatureUsage updates the feature usage metric for TracePipelines
func RecordTracePipelineFeatureUsage(feature, pipelineName string, using bool) {
	recordFeatureUsage(TracePipelineFeatureUsage, feature, pipelineName, using)
}

func recordFeatureUsage(metric *prometheus.GaugeVec, feature, pipelineName string, using bool) {
	value := float64(0)
	if using {
		value = float64(1)
	}

	metric.WithLabelValues(feature, pipelineName).Set(value)
}
