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
	subsystemSelfMonitorProber = "self_monitor_prober"
)

const (
	// General Labels

	LabelEndpoint     = "endpoint"
	LabelPipelineName = "name"

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
)

var (
	GeneralPipelineLabels = []string{
		LabelPipelineName,
		LabelEndpoint,
	}
	MetricPipelineFeatures = []string{
		FeatureTransform,
		FeatureFilter,
		FeatureInputOTLP,
		FeatureInputRuntime,
		FeatureInputPrometheus,
		FeatureInputIstio,
	}

	LogPipelineFeatures = []string{
		FeatureTransform,
		FeatureFilter,
		FeatureInputOTLP,
		FeatureInputRuntime,
		FeatureFilters,
		FeatureOutputCustom,
		FeatureOutputHTTP,
		FeatureVariables,
		FeatureFiles,
	}

	TracePipelineFeatures = []string{
		FeatureTransform,
		FeatureFilter,
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

	MetricPipelineInfo = promauto.With(registry).NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: defaultNamespace,
			Subsystem: subsystemPipelines,
			Name:      "metric_pipeline_info",
			Help:      "Feature and endpoint information of MetricPipelines",
		},
		append(GeneralPipelineLabels, MetricPipelineFeatures...),
	)

	LogPipelineInfo = promauto.With(registry).NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: defaultNamespace,
			Subsystem: subsystemPipelines,
			Name:      "log_pipeline_info",
			Help:      "Feature and endpoint information of LogPipelines",
		},
		append(GeneralPipelineLabels, LogPipelineFeatures...),
	)

	TracePipelineInfo = promauto.With(registry).NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: defaultNamespace,
			Subsystem: subsystemPipelines,
			Name:      "trace_pipeline_info",
			Help:      "Feature and endpoint information of TracePipelines",
		},
		append(GeneralPipelineLabels, TracePipelineFeatures...),
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

func RecordMetricPipelineInfo(pipelineName string, endpoint string, features ...string) {
	recordPipelineInfo(MetricPipelineInfo, MetricPipelineFeatures, pipelineName, endpoint, features...)
}

func RecordLogPipelineInfo(pipelineName string, endpoint string, features ...string) {
	recordPipelineInfo(LogPipelineInfo, LogPipelineFeatures, pipelineName, endpoint, features...)
}

func RecordTracePipelineInfo(pipelineName string, endpoint string, features ...string) {
	recordPipelineInfo(TracePipelineInfo, TracePipelineFeatures, pipelineName, endpoint, features...)
}

func recordPipelineInfo(metric *prometheus.GaugeVec, allFeatures []string, pipelineName string, endpoint string, features ...string) {
	// Create a map of enabled features
	enabledFeatures := make(map[string]bool)
	for _, feature := range features {
		enabledFeatures[feature] = true
	}

	labelValues := []string{pipelineName, endpoint}

	for _, feature := range allFeatures {
		if enabledFeatures[feature] {
			labelValues = append(labelValues, "true")
		} else {
			labelValues = append(labelValues, "false")
		}
	}

	metric.WithLabelValues(labelValues...).Set(1)
}
