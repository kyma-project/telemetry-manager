package common

import (
	"fmt"

	otelports "github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
)

func NewConfig() *Config {
	config := &Config{
		Receivers:  make(map[string]any),
		Processors: make(map[string]any),
		Exporters:  make(map[string]any),
		Connectors: make(map[string]any),
		Extensions: extensionsConfig(),
		Service:    serviceConfig(),
	}

	return config
}

func serviceConfig() Service {
	telemetry := Telemetry{
		Metrics: Metrics{
			Readers: []MetricReader{
				{
					Pull: PullMetricReader{
						Exporter: MetricExporter{
							Prometheus: PrometheusMetricExporter{
								Host: fmt.Sprintf("${%s}", EnvVarCurrentPodIP),
								Port: otelports.Metrics,
							},
						},
					},
				},
			},
		},
		Logs: Logs{
			Level:    "info",
			Encoding: "json",
		},
	}

	return Service{
		Pipelines:  make(map[string]Pipeline),
		Telemetry:  telemetry,
		Extensions: []string{ComponentIDHealthCheckExtension, ComponentIDPprofExtension},
	}
}

func extensionsConfig() map[string]any {
	return map[string]any{
		ComponentIDHealthCheckExtension: Endpoint{
			Endpoint: fmt.Sprintf("${%s}:%d", EnvVarCurrentPodIP, otelports.HealthCheck),
		},
		ComponentIDPprofExtension: Endpoint{
			Endpoint: fmt.Sprintf("127.0.0.1:%d", otelports.Pprof),
		},
	}
}
