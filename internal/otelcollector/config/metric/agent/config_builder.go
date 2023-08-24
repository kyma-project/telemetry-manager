package agent

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
)

type inputSources struct {
	runtime       bool
	prometheus    bool
	istio         bool
	istioDeployed bool
}

func MakeConfig(gatewayServiceName types.NamespacedName, pipelines []v1alpha1.MetricPipeline, istioDeployed bool) *Config {
	inputs := inputSources{
		runtime:       enableRuntimeMetricScraping(pipelines),
		prometheus:    enablePrometheusMetricScraping(pipelines),
		istio:         enableIstioMetricScraping(pipelines),
		istioDeployed: istioDeployed,
	}

	return &Config{
		Base: config.Base{
			Extensions: makeExtensionsConfig(),
			Service:    makeServiceConfig(inputs),
		},
		Receivers:  makeReceiversConfig(inputs),
		Processors: makeProcessorsConfig(inputs),
		Exporters:  makeExportersConfig(gatewayServiceName),
	}
}

func enablePrometheusMetricScraping(pipelines []v1alpha1.MetricPipeline) bool {
	for i := range pipelines {
		input := pipelines[i].Spec.Input
		if input.Application.Prometheus.Enabled {
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

func enableIstioMetricScraping(pipelines []v1alpha1.MetricPipeline) bool {
	for i := range pipelines {
		input := pipelines[i].Spec.Input
		if input.Application.Istio.Enabled {
			return true
		}
	}
	return false
}

func makeExportersConfig(gatewayServiceName types.NamespacedName) Exporters {
	return Exporters{
		OTLP: config.OTLPExporter{
			Endpoint: fmt.Sprintf("%s.%s.svc.cluster.local:%d", gatewayServiceName.Name, gatewayServiceName.Namespace, ports.OTLPGRPC),
			TLS: config.TLS{
				Insecure: true,
			},
			SendingQueue: config.SendingQueue{
				Enabled:   true,
				QueueSize: 512,
			},
			RetryOnFailure: config.RetryOnFailure{
				Enabled:         true,
				InitialInterval: "5s",
				MaxInterval:     "30s",
				MaxElapsedTime:  "300s",
			},
		},
	}
}

func makeExtensionsConfig() config.Extensions {
	return config.Extensions{
		HealthCheck: config.Endpoint{
			Endpoint: fmt.Sprintf("${%s}:%d", config.EnvVarCurrentPodIP, ports.HealthCheck),
		},
	}
}

func makeServiceConfig(inputs inputSources) config.Service {
	return config.Service{
		Pipelines: makePipelinesConfig(inputs),
		Telemetry: config.Telemetry{
			Metrics: config.Metrics{
				Address: fmt.Sprintf("${%s}:%d", config.EnvVarCurrentPodIP, ports.Metrics),
			},
			Logs: config.Logs{
				Level: "info",
			},
		},
		Extensions: []string{"health_check"},
	}
}

func makePipelinesConfig(inputs inputSources) config.Pipelines {
	pipelinesConfig := make(config.Pipelines)

	if inputs.runtime {
		pipelinesConfig["metrics/runtime"] = config.Pipeline{
			Receivers:  []string{"kubeletstats"},
			Processors: []string{"resource/delete-service-name", "resource/insert-input-source-runtime"},
			Exporters:  []string{"otlp"},
		}
	}

	if inputs.prometheus {
		pipelinesConfig["metrics/prometheus"] = config.Pipeline{
			Receivers:  []string{"prometheus/self", "prometheus/app-pods", "prometheus/app-services"},
			Processors: []string{"resource/delete-service-name", "resource/insert-input-source-prometheus"},
			Exporters:  []string{"otlp"},
		}
	}

	if inputs.istio {
		pipelinesConfig["metrics/istio"] = config.Pipeline{
			Receivers:  []string{"prometheus/istio"},
			Processors: []string{"resource/delete-service-name", "resource/insert-input-source-istio"},
			Exporters:  []string{"otlp"},
		}
	}

	return pipelinesConfig
}
