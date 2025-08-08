package gateway

import (
	"fmt"
	"sort"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/otlpexporter"
)

// Service pipeline assembly

func (b *Builder) addServicePipelines(pipeline *telemetryv1alpha1.TracePipeline) {
	pipelineID := formatTracePipelineID(pipeline.Name)
	exporterID := otlpexporter.ExporterID(pipeline.Spec.Output.OTLP.Protocol, pipeline.Name)

	b.config.Service.Pipelines[pipelineID] = pipelineConfig(pipeline, exporterID)
}

// Pipeline ID formatting functions

func formatTracePipelineID(pipelineName string) string {
	return fmt.Sprintf("traces/%s", pipelineName)
}

func formatTransformProcessorID(pipelineName string) string {
	return fmt.Sprintf("transform/%s", pipelineName)
}

// Pipeline configuration functions

func pipelineConfig(pipeline *telemetryv1alpha1.TracePipeline, exporterIDs ...string) config.Pipeline {
	sort.Strings(exporterIDs)

	processors := []string{
		"memory_limiter",
		"k8sattributes",
		"istio_noise_filter",
	}

	// Add transform processors if transforms are specified
	if len(pipeline.Spec.Transforms) > 0 {
		processors = append(processors, formatTransformProcessorID(pipeline.Name))
	}

	processors = append(processors,
		"resource/insert-cluster-attributes",
		"service_enrichment",
		"resource/drop-kyma-attributes",
		"batch",
	)

	return config.Pipeline{
		Receivers:  []string{"otlp"},
		Processors: processors,
		Exporters:  exporterIDs,
	}
}
