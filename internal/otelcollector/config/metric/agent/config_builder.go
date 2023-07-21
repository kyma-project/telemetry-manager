package agent

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
)

type inputSources struct {
	runtime   bool
	workloads bool
}

func MakeConfig(gatewayServiceName types.NamespacedName, pipelines []v1alpha1.MetricPipeline) *Config {
	inputs := inputSources{
		runtime:   enableRuntimeMetricScraping(pipelines),
		workloads: enableWorkloadMetricScraping(pipelines),
	}

	return &Config{
		BaseConfig: config.BaseConfig{
			Extensions: makeExtensionsConfig(),
			Service:    makeServiceConfig(inputs),
		},
		Receivers:  makeReceiversConfig(inputs),
		Processors: makeProcessorsConfig(inputs),
		Exporters:  makeExportersConfig(gatewayServiceName),
	}
}

func enableWorkloadMetricScraping(pipelines []v1alpha1.MetricPipeline) bool {
	for i := range pipelines {
		input := pipelines[i].Spec.Input
		if input.Application.Workloads.Enabled {
			return true
		}
	}
	return false
}

func enableRuntimeMetricScraping(pipelines []v1alpha1.MetricPipeline) bool {
	for i := range pipelines {
		input := pipelines[i].Spec.Input
		if input.Application.Runtime.Enabled {
			return true
		}
	}
	return false
}

func makeExportersConfig(gatewayServiceName types.NamespacedName) ExportersConfig {
	return ExportersConfig{
		OTLP: config.OTLPExporterConfig{
			Endpoint: fmt.Sprintf("%s.%s.svc.cluster.local:%d", gatewayServiceName.Name, gatewayServiceName.Namespace, ports.OTLPGRPC),
			TLS: config.TLSConfig{
				Insecure: true,
			},
			SendingQueue: config.SendingQueueConfig{
				Enabled:   true,
				QueueSize: 512,
			},
			RetryOnFailure: config.RetryOnFailureConfig{
				Enabled:         true,
				InitialInterval: "5s",
				MaxInterval:     "30s",
				MaxElapsedTime:  "300s",
			},
		},
	}
}

func makeExtensionsConfig() config.ExtensionsConfig {
	return config.ExtensionsConfig{
		HealthCheck: config.EndpointConfig{
			Endpoint: fmt.Sprintf("${%s}:%d", config.EnvVarCurrentPodIP, ports.HealthCheck),
		},
	}
}

func makeServiceConfig(inputs inputSources) config.ServiceConfig {
	return config.ServiceConfig{
		Pipelines: makePipelinesConfig(inputs),
		Telemetry: config.TelemetryConfig{
			Metrics: config.MetricsConfig{
				Address: fmt.Sprintf("${%s}:%d", config.EnvVarCurrentPodIP, ports.Metrics),
			},
			Logs: config.LoggingConfig{
				Level: "info",
			},
		},
		Extensions: []string{"health_check"},
	}
}

func makePipelinesConfig(inputs inputSources) config.PipelinesConfig {
	pipelinesConfig := make(config.PipelinesConfig)

	if inputs.runtime {
		pipelinesConfig["metrics/runtime"] = config.PipelineConfig{
			Receivers:  []string{"kubeletstats"},
			Processors: []string{"resource/delete-service-name", "resource/insert-input-source-runtime"},
			Exporters:  []string{"otlp"},
		}
	}

	if inputs.workloads {
		pipelinesConfig["metrics/workloads"] = config.PipelineConfig{
			Receivers:  []string{"prometheus/self", "prometheus/app-pods"},
			Processors: []string{"resource/delete-service-name", "resource/insert-input-source-workloads"},
			Exporters:  []string{"otlp"},
		}
	}

	return pipelinesConfig
}
