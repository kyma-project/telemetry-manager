package agent

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
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
		BaseConfig: common.BaseConfig{
			Extensions: makeExtensionsConfig(),
			Service:    makeServiceConfig(inputDesc),
		},
		Receivers:  makeReceiversConfig(inputDesc),
		Processors: makeProcessorsConfig(),
		Exporters:  makeExportersConfig(gatewayServiceName),
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

func makeExportersConfig(gatewayServiceName types.NamespacedName) common.ExportersConfig {
	exportersConfig := make(common.ExportersConfig)
	exportersConfig["otlp"] = common.ExporterConfig{
		OTLPExporterConfig: &common.OTLPExporterConfig{
			Endpoint: fmt.Sprintf("%s.%s.svc.cluster.local:%d", gatewayServiceName.Name, gatewayServiceName.Namespace, common.PortOTLPGRPC),
			TLS: common.TLSConfig{
				Insecure: true,
			},
			SendingQueue: common.SendingQueueConfig{
				Enabled:   true,
				QueueSize: 512,
			},
			RetryOnFailure: common.RetryOnFailureConfig{
				Enabled:         true,
				InitialInterval: "5s",
				MaxInterval:     "30s",
				MaxElapsedTime:  "300s",
			},
		},
	}
	return exportersConfig
}

func makeExtensionsConfig() common.ExtensionsConfig {
	return common.ExtensionsConfig{
		HealthCheck: common.EndpointConfig{
			Endpoint: fmt.Sprintf("${%s}:%d", common.EnvVarCurrentPodIP, common.PortHealthCheck),
		},
	}
}

func makeServiceConfig(inputDesc inputDescriptor) common.ServiceConfig {
	return common.ServiceConfig{
		Pipelines: makePipelinesConfig(inputDesc),
		Telemetry: common.TelemetryConfig{
			Metrics: common.MetricsConfig{
				Address: fmt.Sprintf("${%s}:%d", common.EnvVarCurrentPodIP, common.PortMetrics),
			},
			Logs: common.LoggingConfig{
				Level: "info",
			},
		},
		Extensions: []string{"health_check"},
	}
}

func makePipelinesConfig(inputDesc inputDescriptor) common.PipelinesConfig {
	pipelinesConfig := make(common.PipelinesConfig)

	if inputDesc.enableRuntimeScraping {
		pipelinesConfig["metrics/runtime"] = common.PipelineConfig{
			Receivers:  []string{"kubeletstats"},
			Processors: []string{"resource/drop-service-name", "resource/emitted-by-runtime"},
			Exporters:  []string{"otlp"},
		}
	}

	if inputDesc.enableWorkloadScraping {
		pipelinesConfig["metrics/workloads"] = common.PipelineConfig{
			Receivers:  []string{"prometheus/self", "prometheus/app-pods"},
			Processors: []string{"resource/drop-service-name", "resource/emitted-by-workloads"},
			Exporters:  []string{"otlp"},
		}
	}

	return pipelinesConfig
}
