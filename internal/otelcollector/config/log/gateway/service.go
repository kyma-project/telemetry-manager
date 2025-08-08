package gateway

import (
	"fmt"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/otlpexporter"
	logpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/logpipeline"
)

// Service pipeline assembly

func (b *Builder) addServicePipelines(pipeline *telemetryv1alpha1.LogPipeline) {
	processorIDs := []string{
		// memory_limiter is always the first processor in the pipeline
		"memory_limiter",
		// Record observed time at the beginning of the pipeline
		"transform/set-observed-time-if-zero",
		"k8sattributes",
		"istio_noise_filter",
	}

	if !logpipelineutils.IsOTLPInputEnabled(pipeline.Spec.Input) {
		processorIDs = append(processorIDs, "filter/drop-if-input-source-otlp")
	}

	// Add namespace filters after k8sattributes processor because they depend on the
	// k8s.namespace.name resource attribute
	if pipeline.Spec.Input.OTLP != nil && !pipeline.Spec.Input.OTLP.Disabled && shouldFilterByNamespace(pipeline.Spec.Input.OTLP.Namespaces) {
		processorIDs = append(processorIDs, formatNamespaceFilterID(pipeline.Name))
	}

	processorIDs = append(processorIDs,
		"resource/insert-cluster-attributes",
		"service_enrichment",
		"resource/drop-kyma-attributes",
		"istio_enrichment",
	)

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

	pipelineID := formatLogPipelineID(pipeline.Name)
	b.config.Service.Pipelines[pipelineID] = pipelineConfig
}

// Pipeline ID formatting functions

func formatLogPipelineID(pipelineName string) string {
	return fmt.Sprintf("logs/%s", pipelineName)
}

func shouldFilterByNamespace(namespaceSelector *telemetryv1alpha1.NamespaceSelector) bool {
	return namespaceSelector != nil && (len(namespaceSelector.Include) > 0 || len(namespaceSelector.Exclude) > 0)
}

func formatNamespaceFilterID(pipelineName string) string {
	return fmt.Sprintf("filter/%s-filter-by-namespace", pipelineName)
}

func formatUserDefinedTransformProcessorID(pipelineName string) string {
	return fmt.Sprintf("transform/user-defined-%s", pipelineName)
}

func formatOTLPExporterID(pipeline *telemetryv1alpha1.LogPipeline) string {
	return otlpexporter.ExporterID(pipeline.Spec.Output.OTLP.Protocol, pipeline.Name)
}
