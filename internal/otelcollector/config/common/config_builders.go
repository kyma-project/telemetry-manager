package common

import (
	"fmt"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
)

// =============================================================================
// BASE CONFIGURATION BUILDERS
// =============================================================================

// DefaultBaseConfig creates a base configuration with the given pipelines and options
func DefaultBaseConfig(pipelines Pipelines, opts ...ConfigOption) Base {
	baseConfig := Base{
		DefaultExtensions(),
		DefaultService(pipelines),
	}

	for _, opt := range opts {
		opt(&baseConfig)
	}

	return baseConfig
}

// ConfigOption defines a function type for modifying base configuration
type ConfigOption func(*Base)

// WithK8sLeaderElector adds Kubernetes leader elector configuration
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

// DefaultService creates a default service configuration with the given pipelines
func DefaultService(pipelines Pipelines) Service {
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

// DefaultExtensions creates default extensions configuration
func DefaultExtensions() Extensions {
	return Extensions{
		HealthCheck: Endpoint{
			Endpoint: fmt.Sprintf("${%s}:%d", EnvVarCurrentPodIP, ports.HealthCheck),
		},
		Pprof: Endpoint{
			Endpoint: fmt.Sprintf("127.0.0.1:%d", ports.Pprof),
		},
	}
}
