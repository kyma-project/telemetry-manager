package tracegateway

import (
	"fmt"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/otlpexporter"
)

// Service pipeline assembly

func (b *Builder) addServicePipelines(pipeline *telemetryv1alpha1.TracePipeline) {
	processorIDs := []string{
		// memory_limiter is always the first processor in the pipeline
		"memory_limiter",
		"k8sattributes",
		"istio_noise_filter",
		"resource/insert-cluster-attributes",
		"service_enrichment",
		"resource/drop-kyma-attributes",
	}

	// Add user-defined transform processor after all of the enrichment processors
	// if transforms are specified
	if len(pipeline.Spec.Transforms) > 0 {
		processorIDs = append(processorIDs, formatUserDefinedTransformProcessorID(pipeline.Name))
	}

	processorIDs = append(processorIDs,
		// batch processor is always the last processor in the pipeline
		"batch",
	)

	pipelineConfig := config.Pipeline{
		Receivers:  []string{"otlp"},
		Processors: processorIDs,
		Exporters:  []string{formatOTLPExporterID(pipeline)},
	}

	pipelineID := formatTracePipelineID(pipeline.Name)
	b.config.Service.Pipelines[pipelineID] = pipelineConfig
}

// Pipeline ID formatting functions

func formatTracePipelineID(pipelineName string) string {
	return fmt.Sprintf("traces/%s", pipelineName)
}

func formatUserDefinedTransformProcessorID(pipelineName string) string {
	return fmt.Sprintf("transform/user-defined-%s", pipelineName)
}

func formatOTLPExporterID(pipeline *telemetryv1alpha1.TracePipeline) string {
	return otlpexporter.ExporterID(pipeline.Spec.Output.OTLP.Protocol, pipeline.Name)
}
