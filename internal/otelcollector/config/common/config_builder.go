package common

import (
	"fmt"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
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

// DisableGoMemLimit disables the GoMemLimit setting in the cgroupruntime extension.
// This should be called when VPA is active, since VPA dynamically adjusts memory limits
// and GoMemLimit would interfere with that.
// TODO: Remove the disabling of GoMemLimit after implementing https://github.com/kyma-project/telemetry-manager/issues/3062
func (c *Config) DisableGoMemLimit() {
	c.Extensions[ComponentIDCGroupRuntimeExtension] = CGroupRuntimeExtension{
		GoMaxProcs: CGroupRuntimeGoMaxProcs{Enabled: false},
		GoMemLimit: CGroupRuntimeGoMemLimit{Enabled: false},
	}
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
		Pipelines:  make(map[string]Pipeline),
		Telemetry:  telemetry,
		Extensions: []string{ComponentIDHealthCheckExtension, ComponentIDPprofExtension, ComponentIDCGroupRuntimeExtension},
	}
}

func extensionsConfig() map[string]any {
	return map[string]any{
		ComponentIDHealthCheckExtension: Endpoint{
			Endpoint: fmt.Sprintf("${%s}:%d", EnvVarCurrentPodIP, ports.HealthCheck),
		},
		ComponentIDPprofExtension: Endpoint{
			Endpoint: fmt.Sprintf("127.0.0.1:%d", ports.Pprof),
		},
		ComponentIDCGroupRuntimeExtension: CGroupRuntimeExtension{
			GoMaxProcs: CGroupRuntimeGoMaxProcs{Enabled: false},
			GoMemLimit: CGroupRuntimeGoMemLimit{Enabled: true, Ratio: 0.8}, //nolint:mnd // 80% of cgroup memory limit
		},
	}
}
