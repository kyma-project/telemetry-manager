package agent

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/builder/common"
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
		BaseConfig: common.BaseConfig{
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
		OTLP: common.OTLPExporterConfig{
			Endpoint: fmt.Sprintf("%s.%s.svc.cluster.local:%d", gatewayServiceName.Name, gatewayServiceName.Namespace, ports.OTLPGRPC),
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
}

func makeExtensionsConfig() common.ExtensionsConfig {
	return common.ExtensionsConfig{
		HealthCheck: common.EndpointConfig{
			Endpoint: fmt.Sprintf("${%s}:%d", common.EnvVarCurrentPodIP, ports.HealthCheck),
		},
	}
}

func makeServiceConfig(inputs inputSources) common.ServiceConfig {
	return common.ServiceConfig{
		Pipelines: makePipelinesConfig(inputs),
		Telemetry: common.TelemetryConfig{
			Metrics: common.MetricsConfig{
				Address: fmt.Sprintf("${%s}:%d", common.EnvVarCurrentPodIP, ports.Metrics),
			},
			Logs: common.LoggingConfig{
				Level: "info",
			},
		},
		Extensions: []string{"health_check"},
	}
}

func makePipelinesConfig(inputs inputSources) common.PipelinesConfig {
	pipelinesConfig := make(common.PipelinesConfig)

	if inputs.runtime {
		pipelinesConfig["metrics/runtime"] = common.PipelineConfig{
			Receivers:  []string{"kubeletstats"},
			Processors: []string{"resource/delete-service-name", "resource/insert-input-source-runtime"},
			Exporters:  []string{"otlp"},
		}
	}

	if inputs.workloads {
		pipelinesConfig["metrics/workloads"] = common.PipelineConfig{
			Receivers:  []string{"prometheus/self", "prometheus/app-pods"},
			Processors: []string{"resource/delete-service-name", "resource/insert-input-source-workloads"},
			Exporters:  []string{"otlp"},
		}
	}

	return pipelinesConfig
}
