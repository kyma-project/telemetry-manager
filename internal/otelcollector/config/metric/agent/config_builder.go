package agent

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
)

type inputSources struct {
	runtime    bool
	prometheus bool
	istio      bool
}

func MakeConfig(gatewayServiceName types.NamespacedName, pipelines []telemetryv1alpha1.MetricPipeline, isIstioActive bool) *Config {
	inputs := inputSources{
		runtime:    enableRuntimeMetricScraping(pipelines),
		prometheus: enablePrometheusMetricScraping(pipelines),
		istio:      enableIstioMetricScraping(pipelines),
	}

	return &Config{
		Base: config.Base{
			Service:    config.DefaultService(makePipelinesConfig(inputs)),
			Extensions: config.DefaultExtensions(),
		},
		Receivers:  makeReceiversConfig(inputs, isIstioActive),
		Processors: makeProcessorsConfig(inputs),
		Exporters:  makeExportersConfig(gatewayServiceName),
	}
}

func enablePrometheusMetricScraping(pipelines []telemetryv1alpha1.MetricPipeline) bool {
	for i := range pipelines {
		input := pipelines[i].Spec.Input
		if input.Prometheus != nil && input.Prometheus.Enabled {
			return true
		}
	}
	return false
}

func enableRuntimeMetricScraping(pipelines []telemetryv1alpha1.MetricPipeline) bool {
	for i := range pipelines {
		input := pipelines[i].Spec.Input
		if input.Runtime != nil && input.Runtime.Enabled {
			return true
		}
	}
	return false
}

func enableIstioMetricScraping(pipelines []telemetryv1alpha1.MetricPipeline) bool {
	for i := range pipelines {
		input := pipelines[i].Spec.Input
		if input.Istio != nil && input.Istio.Enabled {
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

func makePipelinesConfig(inputs inputSources) config.Pipelines {
	pipelinesConfig := make(config.Pipelines)

	if inputs.runtime {
		pipelinesConfig["metrics/runtime"] = config.Pipeline{
			Receivers:  []string{"kubeletstats"},
			Processors: []string{"memory_limiter", "resource/delete-service-name", "resource/insert-input-source-runtime", "transform/set-instrumentation-scope-runtime", "batch"},
			Exporters:  []string{"otlp"},
		}
	}

	if inputs.prometheus {
		pipelinesConfig["metrics/prometheus"] = config.Pipeline{
			Receivers:  []string{"prometheus/app-pods", "prometheus/app-services"},
			Processors: []string{"memory_limiter", "resource/delete-service-name", "resource/insert-input-source-prometheus", "transform/set-instrumentation-scope-prometheus", "batch"},
			Exporters:  []string{"otlp"},
		}
	}

	if inputs.istio {
		pipelinesConfig["metrics/istio"] = config.Pipeline{
			Receivers:  []string{"prometheus/istio"},
			Processors: []string{"memory_limiter", "filter/drop-internal-communication", "resource/delete-service-name", "resource/insert-input-source-istio", "transform/set-instrumentation-scope-istio", "batch"},
			Exporters:  []string{"otlp"},
		}
	}

	return pipelinesConfig
}
