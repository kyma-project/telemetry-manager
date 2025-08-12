package common

import (
	"fmt"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
)

// =============================================================================
// BASE CONFIGURATION BUILDERS
// =============================================================================

type ConfigOption func(*Base)

func WithK8sLeaderElector(authType, leaseName, leaseNamespace string) ConfigOption {
	return func(baseConfig *Base) {
		baseConfig.Service.Extensions = append(baseConfig.Service.Extensions, "k8s_leader_elector")
		baseConfig.Extensions.K8sLeaderElector = K8sLeaderElector{
			AuthType:       authType,
			LeaseName:      leaseName,
			LeaseNamespace: leaseNamespace,
		}
	}
}

func BaseConfig(pipelines Pipelines, opts ...ConfigOption) Base {
	baseConfig := Base{
		ExtensionsConfig(),
		ServiceConfig(pipelines),
	}

	for _, opt := range opts {
		opt(&baseConfig)
	}

	return baseConfig
}

func ServiceConfig(pipelines Pipelines) Service {
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
		Pipelines:  pipelines,
		Telemetry:  telemetry,
		Extensions: []string{"health_check", "pprof"},
	}
}

func ExtensionsConfig() Extensions {
	return Extensions{
		HealthCheck: Endpoint{
			Endpoint: fmt.Sprintf("${%s}:%d", EnvVarCurrentPodIP, ports.HealthCheck),
		},
		Pprof: Endpoint{
			Endpoint: fmt.Sprintf("127.0.0.1:%d", ports.Pprof),
		},
	}
}
