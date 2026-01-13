package metrics

import (
	"context"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/build"
	sharedtypesutils "github.com/kyma-project/telemetry-manager/internal/utils/sharedtypes"
)

const (
	defaultNamespace           = "telemetry"
	subsystemPipelines         = "pipelines"
	subsystemSelfMonitorProber = "self_monitor_prober"
)

const (
	// General features

	FeatureTransform = "transform"
	FeatureFilter    = "filter"
	FeatureInputOTLP = "input-otlp"

	// LogPipeline & MetricPipeline features

	FeatureInputRuntime = "input-runtime"

	// MetricPipeline features

	FeatureInputPrometheus = "input-prometheus"
	FeatureInputIstio      = "input-istio"

	// FluentBit features

	FeatureOutputHTTP   = "output-http"
	FeatureOutputCustom = "output-custom"
	FeatureFiles        = "files"
	FeatureVariables    = "variables"
	FeatureFilters      = "filters-custom"

	// Backends

	FeatureBackendCloudLogging = "backend-cloud-logging"
	FeatureBackendDynatrace    = "backend-dynatrace"
	FeatureBackendLoki         = "backend-loki"
	FeatureBackendElastic      = "backend-elastic"
	FeatureBackendOpenSearch   = "backend-opensearch"
	FeatureBackendSplunk       = "backend-splunk"
	FeatureBackendCloudWatch   = "backend-cloudwatch"
)

// backendPatterns maps backend feature names to their detection patterns
var backendPatterns = map[string][]string{

	FeatureBackendCloudLogging: {"cloud.logs.services.sap.hana.ondemand.com"},
	FeatureBackendDynatrace:    {"dynatrace", "apm.services.cloud.sap"},
	FeatureBackendLoki:         {"loki"},
	FeatureBackendElastic:      {"elastic"},
	FeatureBackendOpenSearch:   {"opensearch"},
	FeatureBackendSplunk:       {"splunk", "log.cdd.net.sap"},
}

var (
	AllFeatures = []string{
		FeatureTransform,
		FeatureFilter,
		FeatureInputOTLP,
		FeatureInputRuntime,
		FeatureFilters,
		FeatureOutputCustom,
		FeatureOutputHTTP,
		FeatureVariables,
		FeatureFiles,
		FeatureBackendLoki,
		FeatureBackendCloudLogging,
		FeatureBackendDynatrace,
		FeatureBackendElastic,
		FeatureBackendOpenSearch,
		FeatureBackendSplunk,
		FeatureBackendCloudWatch,
	}

	registry = metrics.Registry
)

var (
	BuildInfo = promauto.With(registry).NewGauge(
		prometheus.GaugeOpts{
			Namespace:   defaultNamespace,
			Subsystem:   "",
			Name:        "build_info",
			Help:        "Build information of the Telemetry Manager",
			ConstLabels: build.InfoMap(),
		},
	)

	FeatureFlagsInfo = promauto.With(registry).NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: defaultNamespace,
			Name:      "feature_flags_info",
			Help:      "Enabled feature flags in the Telemetry Manager",
		},
		[]string{"flag"},
	)

	MetricPipelineFeatureUsage = promauto.With(registry).NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: defaultNamespace,
			Subsystem: subsystemPipelines,
			Name:      "metric_pipeline_feature_usage",
			Help:      "Number of MetricPipelines using specific features",
		},
		[]string{"feature", "pipeline_name"},
	)

	LogPipelineFeatureUsage = promauto.With(registry).NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: defaultNamespace,
			Subsystem: subsystemPipelines,
			Name:      "log_pipeline_feature_usage",
			Help:      "Number of LogPipelines using specific features",
		},
		[]string{"feature", "pipeline_name"},
	)

	TracePipelineFeatureUsage = promauto.With(registry).NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: defaultNamespace,
			Subsystem: subsystemPipelines,
			Name:      "trace_pipeline_feature_usage",
			Help:      "Number of TracePipelines using specific features",
		},
		[]string{"feature", "pipeline_name"},
	)

	SelfMonitorProberRequestsInFlight = promauto.With(registry).NewGauge(
		prometheus.GaugeOpts{
			Namespace: defaultNamespace,
			Subsystem: subsystemSelfMonitorProber,
			Name:      "in_flight_requests",
			Help:      "The current number of in-flight requests initiated by the self-monitoring prober.",
		},
	)

	SelfMonitorProberRequestsTotal = promauto.With(registry).NewCounterVec(
		prometheus.CounterOpts{
			Namespace: defaultNamespace,
			Subsystem: subsystemSelfMonitorProber,
			Name:      "requests_total",
			Help:      "Total number of requests initiated by the self-monitoring prober.",
		},
		[]string{"code"},
	)

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

func RecordMetricPipelineFeatureUsage(feature, pipelineName string) {
	recordFeatureUsage(MetricPipelineFeatureUsage, feature, pipelineName)
}

func RecordLogPipelineFeatureUsage(feature, pipelineName string) {
	recordFeatureUsage(LogPipelineFeatureUsage, feature, pipelineName)
}

func RecordTracePipelineFeatureUsage(feature, pipelineName string) {
	recordFeatureUsage(TracePipelineFeatureUsage, feature, pipelineName)
}

func recordFeatureUsage(metric *prometheus.GaugeVec, feature, pipelineName string) {
	metric.WithLabelValues(feature, pipelineName).Set(float64(1))
}

// DetectAndTrackOTLPBackend resolves the OTLP endpoint, detects the backend based on URL patterns,
// and records the backend feature usage metric using the provided callback function.
func DetectAndTrackOTLPBackend(ctx context.Context, c client.Client, endpoint telemetryv1beta1.ValueType, pipelineName string, recordMetric func(feature, pipelineName string)) {
	endpointBytes, err := sharedtypesutils.ResolveValue(ctx, c, endpoint)
	if err != nil {
		return
	}

	backend := DetectBackend(string(endpointBytes))
	if backend != "" {
		recordMetric(backend, pipelineName)
	}
}

// DetectBackend identifies the backend type from an endpoint URL by matching against known patterns.
// It performs case-insensitive pattern matching and returns the backend feature name if detected,
// or an empty string if no backend matches.
func DetectBackend(endpoint string) string {
	endpointLower := strings.ToLower(endpoint)

	for backend, patterns := range backendPatterns {
		for _, pattern := range patterns {
			if strings.Contains(endpointLower, pattern) {
				return backend
			}
		}
	}

	return ""
}
