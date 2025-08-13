package agent

import (
	"fmt"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/otlpexporter"
)

// Service pipeline assembly

func (b *Builder) addServicePipelines(pipeline *telemetryv1alpha1.LogPipeline) {
	receiverID := formatFileLogReceiverID(pipeline.Name)
	exporterID := formatOTLPExporterID(pipeline)

	processorIDs := []string{
		// memory_limiter is always the first processor in the pipeline
		"memory_limiter",
		"transform/set-instrumentation-scope-runtime",
		"k8sattributes",
		"resource/insert-cluster-attributes",
		"service_enrichment",
		"resource/drop-kyma-attributes",
		// no batch processor, since pre-batching is performed by the filelog receiver
	}

	// Add user-defined transform processor after all of the enrichment processors
	// if transforms are specified
	if len(pipeline.Spec.Transforms) > 0 {
		processorIDs = append(processorIDs, formatUserDefinedTransformProcessorID(pipeline.Name))
	}

	pipelineConfig := config.Pipeline{
		Receivers:  []string{receiverID},
		Processors: processorIDs,
		Exporters:  []string{exporterID},
	}

	pipelineID := formatLogPipelineID(pipeline.Name)
	b.config.Service.Pipelines[pipelineID] = pipelineConfig
}

// Pipeline ID formatting functions

func formatLogPipelineID(pipelineName string) string {
	return fmt.Sprintf("logs/%s", pipelineName)
}

func formatFileLogReceiverID(pipelineName string) string {
	return fmt.Sprintf("filelog/%s", pipelineName)
}

func formatUserDefinedTransformProcessorID(pipelineName string) string {
	return fmt.Sprintf("transform/user-defined-%s", pipelineName)
}

func formatOTLPExporterID(pipeline *telemetryv1alpha1.LogPipeline) string {
	return otlpexporter.ExporterID(pipeline.Spec.Output.OTLP.Protocol, pipeline.Name)
}
