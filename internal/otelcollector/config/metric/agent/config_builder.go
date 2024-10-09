package agent

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
)

type BuilderConfig struct {
	GatewayOTLPServiceName types.NamespacedName
}

type Builder struct {
	Config BuilderConfig
}

type inputSources struct {
	runtime          bool
	runtimeResources runtimeResourcesEnabled
	prometheus       bool
	istio            bool
}

type runtimeResourcesEnabled struct {
	pod       bool
	container bool
	node      bool
	volume    bool
}

type BuildOptions struct {
	IstioEnabled                bool
	IstioCertPath               string
	InstrumentationScopeVersion string
}

func (b *Builder) Build(pipelines []telemetryv1alpha1.MetricPipeline, opts BuildOptions) *Config {
	inputs := inputSources{
		runtime:          enableRuntimeMetricsScraping(pipelines),
		runtimeResources: enableRuntimeResourcesMetricsScraping(pipelines),
		prometheus:       enablePrometheusMetricsScraping(pipelines),
		istio:            enableIstioMetricsScraping(pipelines),
	}

	return &Config{
		Base: config.Base{
			Service:    config.DefaultService(makePipelinesConfig(inputs)),
			Extensions: config.DefaultExtensions(),
		},
		Receivers:  makeReceiversConfig(inputs, opts),
		Processors: makeProcessorsConfig(inputs, opts.InstrumentationScopeVersion),
		Exporters:  makeExportersConfig(b.Config.GatewayOTLPServiceName),
	}
}

func enableRuntimeMetricsScraping(pipelines []telemetryv1alpha1.MetricPipeline) bool {
	for i := range pipelines {
		input := pipelines[i].Spec.Input
		if input.Runtime != nil && input.Runtime.Enabled {
			return true
		}
	}
	return false
}

func enableRuntimeResourcesMetricsScraping(pipelines []telemetryv1alpha1.MetricPipeline) runtimeResourcesEnabled {
	return runtimeResourcesEnabled{
		pod:       enableRuntimePodMetricsScraping(pipelines),
		container: enableRuntimeContainerMetricsScraping(pipelines),
		node:      enableRuntimeNodeMetricsScraping(pipelines),
		volume:    enableRuntimeVolumeMetricsScraping(pipelines),
	}
}

func enableRuntimePodMetricsScraping(pipelines []telemetryv1alpha1.MetricPipeline) bool {
	for i := range pipelines {
		input := pipelines[i].Spec.Input
		if metric.IsRuntimeInputEnabled(input) && metric.IsRuntimePodMetricsEnabled(input) {
			return true
		}
	}
	return false
}

func enableRuntimeContainerMetricsScraping(pipelines []telemetryv1alpha1.MetricPipeline) bool {
	for i := range pipelines {
		input := pipelines[i].Spec.Input
		if metric.IsRuntimeInputEnabled(input) && metric.IsRuntimeContainerMetricsEnabled(input) {
			return true
		}
	}
	return false
}

func enableRuntimeNodeMetricsScraping(pipelines []telemetryv1alpha1.MetricPipeline) bool {
	for i := range pipelines {
		input := pipelines[i].Spec.Input
		if metric.IsRuntimeInputEnabled(input) && metric.IsRuntimeNodeMetricsEnabled(input) {
			return true
		}
	}
	return false
}

func enableRuntimeVolumeMetricsScraping(pipelines []telemetryv1alpha1.MetricPipeline) bool {
	for i := range pipelines {
		input := pipelines[i].Spec.Input
		if metric.IsRuntimeInputEnabled(input) && metric.IsRuntimeVolumeMetricsEnabled(input) {
			return true
		}
	}
	return false
}

func enablePrometheusMetricsScraping(pipelines []telemetryv1alpha1.MetricPipeline) bool {
	for i := range pipelines {
		input := pipelines[i].Spec.Input
		if input.Prometheus != nil && input.Prometheus.Enabled {
			return true
		}
	}
	return false
}

func enableIstioMetricsScraping(pipelines []telemetryv1alpha1.MetricPipeline) bool {
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
			Processors: makeRuntimePipelineProcessorsIDs(inputs.runtimeResources),
			Exporters:  []string{"otlp"},
		}
	}

	if inputs.prometheus {
		pipelinesConfig["metrics/prometheus"] = config.Pipeline{
			Receivers:  []string{"prometheus/app-pods", "prometheus/app-services"},
			Processors: []string{"memory_limiter", "resource/delete-service-name", "transform/set-instrumentation-scope-prometheus", "batch"},
			Exporters:  []string{"otlp"},
		}
	}

	if inputs.istio {
		pipelinesConfig["metrics/istio"] = config.Pipeline{
			Receivers:  []string{"prometheus/istio"},
			Processors: []string{"memory_limiter", "filter/drop-internal-communication", "resource/delete-service-name", "transform/set-instrumentation-scope-istio", "batch"},
			Exporters:  []string{"otlp"},
		}
	}

	return pipelinesConfig
}

func makeRuntimePipelineProcessorsIDs(runtimeResources runtimeResourcesEnabled) []string {
	processors := []string{"memory_limiter"}

	if runtimeResources.volume {
		processors = append(processors, "filter/drop-non-pvc-volumes-metrics")
	}

	processors = append(processors, "resource/delete-service-name", "transform/set-instrumentation-scope-runtime", "transform/insert-skip-enrichment-attribute", "batch")

	return processors
}
