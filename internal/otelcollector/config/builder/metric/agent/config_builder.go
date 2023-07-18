package agent

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/builder/common"
)

type inputDescriptor struct {
	enableRuntimeScraping  bool
	enableWorkloadScraping bool
}

func MakeConfig(gatewayServiceName types.NamespacedName, pipelines []v1alpha1.MetricPipeline) *Config {
	inputDesc := inputDescriptor{
		enableRuntimeScraping:  enableRuntimeMetricScraping(pipelines),
		enableWorkloadScraping: enableWorkloadMetricScraping(pipelines),
	}

	return &Config{
		Receivers:  makeReceiversConfig(inputDesc),
		Processors: makeProcessorsConfig(),
		Exporters:  makeExportersConfig(gatewayServiceName),
		Extensions: makeExtensionsConfig(),
		Service:    makeServiceConfig(inputDesc),
	}
}

func enableWorkloadMetricScraping(pipelines []v1alpha1.MetricPipeline) bool {
	for i := range pipelines {
		input := pipelines[i].Spec.Input
		if input.Application.Workload.Enabled {
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

func makeExportersConfig(gatewayServiceName types.NamespacedName) config.ExportersConfig {
	exportersConfig := make(config.ExportersConfig)
	exportersConfig["otlp"] = config.ExporterConfig{
		OTLPExporterConfig: &config.OTLPExporterConfig{
			Endpoint: fmt.Sprintf("%s.%s.svc.cluster.local:%d", gatewayServiceName.Name, gatewayServiceName.Namespace, common.PortOTLPGRPC),
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
	return exportersConfig
}

func makeExtensionsConfig() config.ExtensionsConfig {
	return config.ExtensionsConfig{
		HealthCheck: config.EndpointConfig{
			Endpoint: fmt.Sprintf("${%s}:%d", common.EnvVarCurrentPodIP, common.PortHealthCheck),
		},
	}
}

func makeServiceConfig(inputDesc inputDescriptor) config.ServiceConfig {
	return config.ServiceConfig{
		Pipelines: makePipelinesConfig(inputDesc),
		Telemetry: config.TelemetryConfig{
			Metrics: config.MetricsConfig{
				Address: fmt.Sprintf("${%s}:%d", common.EnvVarCurrentPodIP, common.PortMetrics),
			},
			Logs: config.LoggingConfig{
				Level: "info",
			},
		},
		Extensions: []string{"health_check"},
	}
}

func makePipelinesConfig(inputDesc inputDescriptor) config.PipelinesConfig {
	pipelinesConfig := make(config.PipelinesConfig)

	if inputDesc.enableRuntimeScraping {
		pipelinesConfig["metrics/runtime"] = config.PipelineConfig{
			Receivers:  []string{"kubeletstats"},
			Processors: []string{"resource/drop-service-name", "resource/emitted-by-runtime"},
			Exporters:  []string{"otlp"},
		}
	}

	if inputDesc.enableWorkloadScraping {
		pipelinesConfig["metrics/workloads"] = config.PipelineConfig{
			Receivers:  []string{"prometheus/self", "prometheus/app-pods"},
			Processors: []string{"resource/drop-service-name", "resource/emitted-by-workloads"},
			Exporters:  []string{"otlp"},
		}
	}

	return pipelinesConfig
}
