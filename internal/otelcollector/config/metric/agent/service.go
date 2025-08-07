package agent

import "github.com/kyma-project/telemetry-manager/internal/otelcollector/config"

func pipelinesConfig(inputs inputSources) config.Pipelines {
	pipelinesCfg := make(config.Pipelines)

	if inputs.runtime {
		pipelinesCfg["metrics/runtime"] = config.Pipeline{
			Receivers:  []string{"kubeletstats", "k8s_cluster"},
			Processors: runtimePipelineProcessorIDs(inputs.runtimeResources),
			Exporters:  []string{"otlp"},
		}
	}

	if inputs.prometheus {
		pipelinesCfg["metrics/prometheus"] = config.Pipeline{
			Receivers:  []string{"prometheus/app-pods", "prometheus/app-services"},
			Processors: []string{"memory_limiter", "resource/delete-service-name", "transform/set-instrumentation-scope-prometheus", "batch"},
			Exporters:  []string{"otlp"},
		}
	}

	if inputs.istio {
		pipelinesCfg["metrics/istio"] = config.Pipeline{
			Receivers:  []string{"prometheus/istio"},
			Processors: []string{"memory_limiter", "istio_noise_filter", "resource/delete-service-name", "transform/set-instrumentation-scope-istio", "batch"},
			Exporters:  []string{"otlp"},
		}
	}

	return pipelinesCfg
}

func runtimePipelineProcessorIDs(runtimeResources runtimeResourceSources) []string {
	processors := []string{"memory_limiter"}

	if runtimeResources.volume {
		processors = append(processors, "filter/drop-non-pvc-volumes-metrics")
	}

	processors = append(processors, "filter/drop-virtual-network-interfaces", "resource/delete-service-name", "transform/set-instrumentation-scope-runtime", "transform/insert-skip-enrichment-attribute", "batch")

	return processors
}
