package agent

import (
	"fmt"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/otlpexporter"
)

// Service pipeline assembly

func (b *Builder) addServicePipelines(pipeline *telemetryv1alpha1.LogPipeline) {
	pipelineID := formatLogPipelineID(pipeline.Name)
	receiverID := formatFileLogReceiverID(pipeline.Name)
	exporterID := otlpexporter.ExporterID(pipeline.Spec.Output.OTLP.Protocol, pipeline.Name)

	b.config.Service.Pipelines[pipelineID] = pipelineConfig(receiverID, exporterID)
}

// Pipeline ID formatting functions

func formatLogPipelineID(pipelineName string) string {
	return fmt.Sprintf("logs/%s", pipelineName)
}

func formatFileLogReceiverID(pipelineName string) string {
	return fmt.Sprintf("filelog/%s", pipelineName)
}

func formatTransformProcessorID(pipelineName string) string {
	return fmt.Sprintf("transform/%s", pipelineName)
}

// Pipeline configuration functions

// Each pipeline will have one receiver and one exporter
func pipelineConfig(receiverID, exporterID string) config.Pipeline {
	return config.Pipeline{
		Receivers: []string{receiverID},
		Processors: []string{
			"memory_limiter",
			"transform/set-instrumentation-scope-runtime",
			"k8sattributes",
			"resource/insert-cluster-attributes",
			"service_enrichment",
			"resource/drop-kyma-attributes",
		},
		Exporters: []string{exporterID},
	}
}
