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

	b.config.Service.Pipelines[pipelineID] = pipelineConfig(exporterID)
}

// Pipeline ID formatting functions

func formatTracePipelineID(pipelineName string) string {
	return fmt.Sprintf("traces/%s", pipelineName)
}

// Pipeline configuration functions

func pipelineConfig(exporterIDs ...string) config.Pipeline {
	sort.Strings(exporterIDs)

	return config.Pipeline{
		Receivers: []string{"otlp"},
		Processors: []string{
			"memory_limiter",
			"k8sattributes",
			"istio_noise_filter",
			"resource/insert-cluster-attributes",
			"service_enrichment",
			"resource/drop-kyma-attributes",
			"batch",
		},
		Exporters: exporterIDs,
	}
}
