package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/kyma-project/telemetry-manager/internal/build"
)

const (
	defaultNamespace           = "telemetry"
	subsystemPipelines         = "pipelines"
	subsystemSelfMonitorProber = "self_monitor_prober_"
)

// registry is the prometheus registry for telemetry manager metrics
var registry = ctrlmetrics.Registry

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

	// OTTLTransformUsage tracks the number of pipelines using OTTL transform feature
	OTTLTransformUsage = promauto.With(registry).NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: defaultNamespace,
			Subsystem: subsystemPipelines,
			Name:      "ottl_transform_usage",
			Help:      "Number of pipelines using OTTL transform feature",
		},
		[]string{"kind"},
	)

	// OTTLFilterUsage tracks the number of pipelines using OTTL filter feature
	OTTLFilterUsage = promauto.With(registry).NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: defaultNamespace,
			Subsystem: subsystemPipelines,
			Name:      "ottl_filter_usage",
			Help:      "Number of pipelines using OTTL filter feature",
		},
		[]string{"pipeline_type"},
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

// RecordOTTLTransformUsage updates the transform usage metric for a given pipeline type
func RecordOTTLTransformUsage(kind string, count int) {
	OTTLTransformUsage.WithLabelValues(kind).Set(float64(count))
}

// RecordOTTLFilterUsage updates the filter usage metric for a given pipeline type
func RecordOTTLFilterUsage(kind string, count int) {
	OTTLFilterUsage.WithLabelValues(kind).Set(float64(count))
}
