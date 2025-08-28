package common

import (
	"fmt"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
)

// =============================================================================
// BASE CONFIGURATION BUILDERS
// =============================================================================

type ConfigOption func(*Config)

func WithK8sLeaderElector(authType, leaseName, leaseNamespace string) ConfigOption {
	return func(config *Config) {
		config.Service.Extensions = append(config.Service.Extensions, "k8s_leader_elector")
		config.Extensions.K8sLeaderElector = K8sLeaderElector{
			AuthType:       authType,
			LeaseName:      leaseName,
			LeaseNamespace: leaseNamespace,
		}
	}
}

func NewConfig(opts ...ConfigOption) *Config {
	config := &Config{
		Receivers:  make(map[string]any),
		Processors: make(map[string]any),
		Exporters:  make(map[string]any),
		Connectors: make(map[string]any),
		Extensions: extensionsConfig(),
		Service:    serviceConfig(),
	}

	for _, opt := range opts {
		opt(config)
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
								Port: ports.Metrics,
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
		Pipelines:  make(Pipelines),
		Telemetry:  telemetry,
		Extensions: []string{"health_check", "pprof"},
	}
}

func extensionsConfig() Extensions {
	return Extensions{
		HealthCheck: Endpoint{
			Endpoint: fmt.Sprintf("${%s}:%d", EnvVarCurrentPodIP, ports.HealthCheck),
		},
		Pprof: Endpoint{
			Endpoint: fmt.Sprintf("127.0.0.1:%d", ports.Pprof),
		},
	}
}
