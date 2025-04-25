package telemetry

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var compatibilityModeGauge = prometheus.NewGauge(prometheus.GaugeOpts{
	Name: "telemetry_otelcol_metrics_compatibility_mode",
	Help: "Indicates if the OpenTelemetry internal metrics compatibility mode is enabled (1) or disabled (0)",
})

type telemetryMetricsEmitter struct{}

//nolint:unparam // No good explanation why this linter is firing here
func newTelemetryMetricsEmitter() telemetryMetricsEmitter {
	metrics.Registry.MustRegister(compatibilityModeGauge)
	return telemetryMetricsEmitter{}
}

func (e telemetryMetricsEmitter) updateCompatibilityModeMetric(compatibilityMode bool) {
	if compatibilityMode {
		compatibilityModeGauge.Set(1)
	} else {
		compatibilityModeGauge.Set(0)
	}
}
