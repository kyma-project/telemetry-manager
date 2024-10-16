package config

import (
	"fmt"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
)

type Base struct {
	Extensions Extensions `yaml:"extensions"`
	Service    Service    `yaml:"service"`
}

type Extensions struct {
	HealthCheck Endpoint `yaml:"health_check,omitempty"`
	Pprof       Endpoint `yaml:"pprof,omitempty"`
}

type Endpoint struct {
	Endpoint string `yaml:"endpoint,omitempty"`
}

type Service struct {
	Pipelines  Pipelines `yaml:"pipelines,omitempty"`
	Telemetry  Telemetry `yaml:"telemetry,omitempty"`
	Extensions []string  `yaml:"extensions,omitempty"`
}

type Pipelines map[string]Pipeline

type Pipeline struct {
	Receivers  []string `yaml:"receivers"`
	Processors []string `yaml:"processors"`
	Exporters  []string `yaml:"exporters"`
}

type Telemetry struct {
	Metrics Metrics `yaml:"metrics"`
	Logs    Logs    `yaml:"logs"`
}

type Metrics struct {
	Readers []MetricReader `yaml:"readers"`
}

type MetricReader struct {
	Pull PullMetricReader `yaml:"pull"`
}

type PullMetricReader struct {
	Exporter MetricExporter `yaml:"exporter"`
}

type MetricExporter struct {
	Prometheus PrometheusMetricExporter `yaml:"prometheus"`
}

type PrometheusMetricExporter struct {
	Host string `yaml:"host"`
	Port int32  `yaml:"port"`
}

type Logs struct {
	Level    string `yaml:"level"`
	Encoding string `yaml:"encoding"`
}

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
